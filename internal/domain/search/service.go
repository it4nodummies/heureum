package search

import (
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/jql"
	"gorm.io/gorm"
)

// Service esegue ricerche JQL sulle issue.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// DB espone la connessione sottostante, usata dagli handler per costruire un
// search.DBResolver legato alla request corrente (es. currentUserID).
func (s *Service) DB() *gorm.DB { return s.db }

// SearchResult porta la pagina di issue e il totale complessivo (per la
// paginazione offset del legacy /search).
type SearchResult struct {
	Issues []issue.Issue
	Total  int
}

// Search compila la JQL e la esegue. jql vuota => tutte le issue non archiviate.
// offset/limit implementano la paginazione. Escludiamo sempre le archiviate.
// scope, se non nil, è una subquery sui project_id visibili al chiamante (es.
// project.Service.MembershipSubquery) usata per limitare i risultati ai
// progetti di cui l'utente è membro; nil = nessuna limitazione (admin globale).
func (s *Service) Search(jqlStr string, r jql.Resolver, offset, limit int, scope *gorm.DB) (SearchResult, error) {
	if offset < 0 {
		offset = 0
	}

	q, err := jql.Parse(jqlStr)
	if err != nil {
		return SearchResult{}, err
	}
	compiled, err := jql.Compile(q, r)
	if err != nil {
		return SearchResult{}, err
	}

	base := s.db.Model(&issue.Issue{}).Where("is_archived = ?", false)
	if scope != nil {
		base = base.Where("issues.project_id IN (?)", scope)
	}
	if compiled.Where != "" {
		base = base.Where(compiled.Where, compiled.Args...)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return SearchResult{}, err
	}

	// limit == 0: solo conteggio (usato da /search/approximate-count) — niente Find.
	if limit == 0 {
		return SearchResult{Total: int(total)}, nil
	}

	q2 := base
	if compiled.Order != "" {
		q2 = q2.Order(compiled.Order)
	} else {
		q2 = q2.Order("seq_id ASC")
	}
	if limit > 0 {
		q2 = q2.Limit(limit)
	}
	if offset > 0 {
		q2 = q2.Offset(offset)
	}

	var issues []issue.Issue
	if err := q2.Find(&issues).Error; err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Issues: issues, Total: int(total)}, nil
}
