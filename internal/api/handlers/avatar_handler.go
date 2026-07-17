package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// maxAvatarBytes è il tetto sulla dimensione dell'immagine avatar (2 MB).
const maxAvatarBytes = 2 << 20

// avatarContentTypeExt mappa i content-type immagine accettati alla loro
// estensione su disco. Il set è volutamente ristretto (png/jpeg/gif/webp):
// http.DetectContentType riconosce tutti questi dai magic bytes.
var avatarContentTypeExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// AvatarHandler gestisce l'upload (autenticato) e il serve (pubblico) degli
// avatar utente. Non riusa AttachmentService perché quello è vincolato a una
// issue (IssueID NOT NULL); gli avatar sono file su disco sotto
// UploadsDir/avatars/ con nome deterministico <uid>.<ext> (il re-upload
// sovrascrive), e l'URL servito è persistito in users.avatar_url.
type AvatarHandler struct {
	DB         *gorm.DB
	UploadsDir string
	BaseURL    string
}

func NewAvatarHandler(db *gorm.DB, uploadsDir, baseURL string) *AvatarHandler {
	return &AvatarHandler{DB: db, UploadsDir: uploadsDir, BaseURL: baseURL}
}

func (h *AvatarHandler) avatarsDir() string {
	return filepath.Join(h.UploadsDir, "avatars")
}

// Upload: POST /rest/api/3/myself/avatar (autenticato). Scrive l'avatar del
// SOLO chiamante (chiave = uid dal context, mai un path param), valida
// content-type immagine + dimensione, e ritorna il v3.User aggiornato.
func (h *AvatarHandler) Upload(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	if uid == "" {
		v3.WriteError(w, http.StatusUnauthorized, []string{"unauthorized"}, nil)
		return
	}

	// Cap the request body BEFORE parsing: ParseMultipartForm spools the full
	// part to a temp file regardless of the in-memory threshold, so without
	// this an authenticated client could force multi-GB temp writes. Slack of
	// 1KB covers the multipart envelope overhead.
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+1024)
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"failed to parse form"}, nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"file is required"}, nil)
		return
	}
	defer file.Close()

	if header.Size > maxAvatarBytes {
		v3.WriteError(w, http.StatusBadRequest, []string{"avatar exceeds 2MB"}, nil)
		return
	}

	// Leggi il contenuto (capato) e determina il content-type reale via sniff
	// sui primi 512 byte: non ci fidiamo del solo header dichiarato dal client.
	data, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"failed to read file"}, nil)
		return
	}
	if len(data) > maxAvatarBytes {
		v3.WriteError(w, http.StatusBadRequest, []string{"avatar exceeds 2MB"}, nil)
		return
	}

	sniffed := http.DetectContentType(data)
	ext, ok := avatarContentTypeExt[sniffed]
	if !ok {
		v3.WriteError(w, http.StatusBadRequest, []string{"unsupported image type; allowed: png, jpeg, gif, webp"}, nil)
		return
	}

	dir := h.avatarsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to store avatar"}, nil)
		return
	}
	// Rimuovi eventuali avatar precedenti con estensione diversa, così il
	// serve (che fa glob su <uid>.*) non trova un file stantìo.
	if old, _ := filepath.Glob(filepath.Join(dir, uid+".*")); old != nil {
		for _, f := range old {
			_ = os.Remove(f)
		}
	}
	dst := filepath.Join(dir, uid+ext)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to store avatar"}, nil)
		return
	}

	servedURL := "/rest/api/3/user/avatar/" + uid
	svc := user.NewService(h.DB)
	u, err := svc.UpdateProfile(uid, nil, nil, nil, &servedURL)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update profile"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraUser(*u, h.BaseURL))
}

// Serve: GET /rest/api/3/user/avatar/{userId} — PUBBLICO (senza authMw, come
// serveDefaultAvatar), così <img src> funziona senza bearer token. Serve il
// file su disco dell'utente; se non esiste, 404.
func (h *AvatarHandler) Serve(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	// Difesa contro path traversal E glob injection: l'id è un uuid/plain id,
	// quindi accettiamo solo [A-Za-z0-9-]. Questo esclude sia i separatori di
	// path (/ \ ..) sia i metacaratteri di glob (* ? [) che altrimenti
	// farebbero servire l'avatar di un altro utente via filepath.Glob.
	if !validAvatarID(userID) {
		http.NotFound(w, r)
		return
	}
	matches, _ := filepath.Glob(filepath.Join(h.avatarsDir(), userID+".*"))
	if len(matches) == 0 {
		http.NotFound(w, r)
		return
	}
	path := matches[0]
	if ct := avatarContentTypeForExt(filepath.Ext(path)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}

// avatarIDPattern accetta solo caratteri di un id/uuid semplice: esclude
// separatori di path e metacaratteri di glob.
var avatarIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func validAvatarID(id string) bool {
	return avatarIDPattern.MatchString(id)
}

// avatarContentTypeForExt inverte avatarContentTypeExt (ext -> content-type).
func avatarContentTypeForExt(ext string) string {
	for ct, e := range avatarContentTypeExt {
		if e == ext {
			return ct
		}
	}
	return ""
}
