package jql

import (
	"fmt"
	"strings"
)

// Parse trasforma una stringa JQL in un *Query. Stringa vuota => Query senza
// condizioni né ordinamento (equivale a "tutte le issue").
func Parse(input string) (*Query, error) {
	toks, err := Lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	q := &Query{}

	// condizione opzionale: presente se non iniziamo con ORDER o EOF
	if !p.at(TokEOF) && !p.atKeyword("ORDER") {
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		q.Where = node
	}

	// ORDER BY opzionale
	if p.atKeyword("ORDER") {
		p.next()
		if !p.atKeyword("BY") {
			return nil, fmt.Errorf("atteso BY dopo ORDER")
		}
		p.next()
		order, err := p.parseOrder()
		if err != nil {
			return nil, err
		}
		q.Order = order
	}

	if !p.at(TokEOF) {
		return nil, fmt.Errorf("token inatteso in coda: %q", p.cur().Val)
	}
	return q, nil
}

type parser struct {
	toks []Token
	pos  int
}

func (p *parser) cur() Token        { return p.toks[p.pos] }
func (p *parser) next() Token       { t := p.toks[p.pos]; p.pos++; return t }
func (p *parser) at(k TokKind) bool { return p.cur().Kind == k }
func (p *parser) atKeyword(k string) bool {
	return p.cur().Kind == TokKeyword && p.cur().Val == k
}

func (p *parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.atKeyword("OR") {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Node, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.atKeyword("AND") {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = And{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Node, error) {
	if p.atKeyword("NOT") {
		p.next()
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return Not{Inner: inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	if p.at(TokLParen) {
		p.next()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.at(TokRParen) {
			return nil, fmt.Errorf("attesa ) di chiusura")
		}
		p.next()
		return node, nil
	}
	return p.parseClause()
}

func (p *parser) parseClause() (Node, error) {
	if !p.at(TokIdent) {
		return nil, fmt.Errorf("atteso nome campo, got %q", p.cur().Val)
	}
	field := strings.ToLower(p.next().Val)
	c := &Clause{Field: field}

	switch {
	case p.at(TokOp):
		c.Op = p.next().Val
		return p.parseScalarValue(c)
	case p.atKeyword("IN"):
		p.next()
		c.Op = "IN"
		return p.parseList(c)
	case p.atKeyword("NOT"):
		p.next()
		if !p.atKeyword("IN") {
			return nil, fmt.Errorf("atteso IN dopo NOT")
		}
		p.next()
		c.Op = "NOT IN"
		return p.parseList(c)
	case p.atKeyword("IS"):
		p.next()
		c.Op = "IS"
		if p.atKeyword("NOT") {
			p.next()
			c.Op = "IS NOT"
		}
		if !p.atKeyword("EMPTY") && !p.atKeyword("NULL") {
			return nil, fmt.Errorf("atteso EMPTY/NULL dopo IS")
		}
		p.next()
		c.IsEmpty = true
		return c, nil
	default:
		return nil, fmt.Errorf("atteso operatore dopo campo %q, got %q", field, p.cur().Val)
	}
}

func (p *parser) parseScalarValue(c *Clause) (Node, error) {
	switch p.cur().Kind {
	case TokIdent:
		val := p.next().Val
		// funzione? es. currentUser()
		if p.at(TokLParen) {
			p.next()
			if !p.at(TokRParen) {
				return nil, fmt.Errorf("solo funzioni senza argomenti supportate: %s", val)
			}
			p.next()
			c.Func = val
			return c, nil
		}
		c.Value = val
		return c, nil
	case TokString, TokNumber:
		c.Value = p.next().Val
		return c, nil
	case TokKeyword:
		if p.atKeyword("EMPTY") || p.atKeyword("NULL") {
			p.next()
			c.IsEmpty = true
			return c, nil
		}
		return nil, fmt.Errorf("valore inatteso %q", p.cur().Val)
	default:
		return nil, fmt.Errorf("atteso valore dopo operatore, got %q", p.cur().Val)
	}
}

func (p *parser) parseList(c *Clause) (Node, error) {
	if !p.at(TokLParen) {
		return nil, fmt.Errorf("attesa ( dopo IN")
	}
	p.next()
	for {
		switch p.cur().Kind {
		case TokIdent, TokString, TokNumber:
			c.Values = append(c.Values, p.next().Val)
		default:
			return nil, fmt.Errorf("valore di lista inatteso %q", p.cur().Val)
		}
		if p.at(TokComma) {
			p.next()
			continue
		}
		break
	}
	if !p.at(TokRParen) {
		return nil, fmt.Errorf("attesa ) di chiusura lista")
	}
	p.next()
	if len(c.Values) == 0 {
		return nil, fmt.Errorf("lista IN vuota")
	}
	return c, nil
}

func (p *parser) parseOrder() ([]OrderKey, error) {
	var keys []OrderKey
	for {
		if !p.at(TokIdent) {
			return nil, fmt.Errorf("atteso campo in ORDER BY, got %q", p.cur().Val)
		}
		k := OrderKey{Field: strings.ToLower(p.next().Val)}
		if p.atKeyword("ASC") {
			p.next()
		} else if p.atKeyword("DESC") {
			p.next()
			k.Desc = true
		}
		keys = append(keys, k)
		if p.at(TokComma) {
			p.next()
			continue
		}
		break
	}
	return keys, nil
}
