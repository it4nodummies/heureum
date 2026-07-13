package jql

// Node è l'interfaccia comune dei nodi dell'albero di condizioni.
type Node interface{ isNode() }

// Query è la radice: una condizione opzionale + ordinamento opzionale.
type Query struct {
	Where Node       // nil se assente (query "tutte le issue")
	Order []OrderKey // vuoto se assente
}

// And / Or / Not compongono le condizioni.
type And struct{ Left, Right Node }
type Or struct{ Left, Right Node }
type Not struct{ Inner Node }

// Clause è una condizione atomica: field OP value.
type Clause struct {
	Field   string
	Op      string   // = != > >= < <= ~ !~ IN "NOT IN" IS "IS NOT"
	Value   string   // per operatori scalari
	Values  []string // per IN / NOT IN
	Func    string   // nome funzione se il valore è una funzione, es. "currentUser"
	IsEmpty bool     // true se il valore è EMPTY/NULL (con IS / IS NOT)
}

// OrderKey è un campo di ordinamento con direzione.
type OrderKey struct {
	Field string
	Desc  bool
}

func (And) isNode()     {}
func (Or) isNode()      {}
func (Not) isNode()     {}
func (*Clause) isNode() {}
