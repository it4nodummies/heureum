package board

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// --- Persistence + domain models ---
//
// BoardColumn e BoardQuickFilter fungono sia da modello GORM (tabelle
// board_columns / board_quick_filters) sia da tipo di dominio esposto verso i
// layer superiori (Task 4). StatusIDs non è una colonna: viene popolato dai
// record BoardColumnStatus (join board_column_statuses) e ignorato dalla
// persistenza via `gorm:"-"`.

// BoardColumn è una colonna configurata della board che mappa un INSIEME di
// stati (StatusIDs), con un ordinamento dato da Position.
type BoardColumn struct {
	ID        string   `gorm:"primaryKey;type:text" json:"id"`
	BoardID   string   `gorm:"column:board_id;type:text;not null;index" json:"-"`
	Name      string   `gorm:"type:text;not null" json:"name"`
	Position  int      `gorm:"not null;default:0" json:"position"`
	StatusIDs []string `gorm:"-" json:"statusIds"`
}

func (BoardColumn) TableName() string { return "board_columns" }

// BoardColumnStatus è il record di join tra una colonna e uno stato.
type BoardColumnStatus struct {
	ColumnID string `gorm:"column:column_id;type:text;primaryKey" json:"-"`
	StatusID string `gorm:"column:status_id;type:text;primaryKey" json:"-"`
}

func (BoardColumnStatus) TableName() string { return "board_column_statuses" }

// BoardQuickFilter è un chip di filtro rapido (nome + JQL) ordinato.
type BoardQuickFilter struct {
	ID       string `gorm:"primaryKey;type:text" json:"id"`
	BoardID  string `gorm:"column:board_id;type:text;not null;index" json:"-"`
	Name     string `gorm:"type:text;not null" json:"name"`
	JQL      string `gorm:"column:jql;type:text;not null" json:"jql"`
	Position int    `gorm:"not null;default:0" json:"position"`
}

func (BoardQuickFilter) TableName() string { return "board_quick_filters" }

// --- Domain aggregates / IO ---

// BoardConfig è la configurazione completa (letta) di una board.
type BoardConfig struct {
	Columns      []BoardColumn      `json:"columns"`
	Swimlane     string             `json:"swimlane"`
	QuickFilters []BoardQuickFilter `json:"quickFilters"`
}

// BoardConfigInput è il payload di scrittura della configurazione.
type BoardConfigInput struct {
	Columns []struct {
		Name      string   `json:"name"`
		StatusIDs []string `json:"statusIds"`
	} `json:"columns"`
	Swimlane     string `json:"swimlane"`
	QuickFilters []struct {
		Name string `json:"name"`
		JQL  string `json:"jql"`
	} `json:"quickFilters"`
}

// FallbackStatus è uno stato di workflow usato per costruire le colonne 1:1 di
// default quando la board non ha una configurazione persistita.
type FallbackStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetConfig restituisce la configurazione persistita della board: colonne
// (ordinate per position, con i rispettivi status id), swimlane mode e quick
// filters (ordinati). Se NON esistono colonne persistite, costruisce le colonne
// 1:1 di default a partire da fallback (una colonna per stato), swimlane "none"
// e nessun filtro.
func (s *Service) GetConfig(boardID string, fallback []FallbackStatus) (BoardConfig, error) {
	var cols []BoardColumn
	if err := s.db.Where("board_id = ?", boardID).Order("position ASC").Find(&cols).Error; err != nil {
		return BoardConfig{}, err
	}

	if len(cols) == 0 {
		def := make([]BoardColumn, len(fallback))
		for i, st := range fallback {
			def[i] = BoardColumn{Name: st.Name, Position: i, StatusIDs: []string{st.ID}}
		}
		return BoardConfig{Columns: def, Swimlane: "none", QuickFilters: []BoardQuickFilter{}}, nil
	}

	for i := range cols {
		var links []BoardColumnStatus
		if err := s.db.Where("column_id = ?", cols[i].ID).Order("status_id ASC").Find(&links).Error; err != nil {
			return BoardConfig{}, err
		}
		ids := make([]string, len(links))
		for j, l := range links {
			ids[j] = l.StatusID
		}
		cols[i].StatusIDs = ids
	}

	var filters []BoardQuickFilter
	if err := s.db.Where("board_id = ?", boardID).Order("position ASC").Find(&filters).Error; err != nil {
		return BoardConfig{}, err
	}

	var b Board
	if err := s.db.Where("id = ?", boardID).First(&b).Error; err != nil {
		return BoardConfig{}, err
	}
	swim := b.SwimlaneMode
	if swim == "" {
		swim = "none"
	}

	return BoardConfig{Columns: cols, Swimlane: swim, QuickFilters: filters}, nil
}

// SaveConfig sostituisce transazionalmente colonne (+link stati) e quick filters
// della board e imposta boards.swimlane_mode. Genera nuovi uuid e assegna le
// position in base all'indice.
func (s *Service) SaveConfig(boardID string, in BoardConfigInput) error {
	swim := in.Swimlane
	if swim == "" {
		swim = "none"
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing []BoardColumn
		if err := tx.Where("board_id = ?", boardID).Find(&existing).Error; err != nil {
			return err
		}
		for _, c := range existing {
			if err := tx.Where("column_id = ?", c.ID).Delete(&BoardColumnStatus{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("board_id = ?", boardID).Delete(&BoardColumn{}).Error; err != nil {
			return err
		}
		if err := tx.Where("board_id = ?", boardID).Delete(&BoardQuickFilter{}).Error; err != nil {
			return err
		}

		for i, c := range in.Columns {
			col := BoardColumn{ID: uuid.NewString(), BoardID: boardID, Name: c.Name, Position: i}
			if err := tx.Create(&col).Error; err != nil {
				return err
			}
			for _, sid := range c.StatusIDs {
				if err := tx.Create(&BoardColumnStatus{ColumnID: col.ID, StatusID: sid}).Error; err != nil {
					return err
				}
			}
		}

		for i, f := range in.QuickFilters {
			qf := BoardQuickFilter{ID: uuid.NewString(), BoardID: boardID, Name: f.Name, JQL: f.JQL, Position: i}
			if err := tx.Create(&qf).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&Board{}).Where("id = ?", boardID).Update("swimlane_mode", swim).Error; err != nil {
			return err
		}
		return nil
	})
}
