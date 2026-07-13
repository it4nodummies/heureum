// Package jql implementa un lexer, parser e compilatore per un sottoinsieme
// realistico di JQL (Jira Query Language) verso SQL/GORM.
package jql

import (
	"fmt"
	"strings"
	"unicode"
)

type TokKind int

const (
	TokEOF TokKind = iota
	TokIdent
	TokString
	TokNumber
	TokOp      // = != > >= < <= ~ !~
	TokKeyword // AND OR NOT IN IS EMPTY NULL ASC DESC ORDER BY
	TokLParen
	TokRParen
	TokComma
)

type Token struct {
	Kind TokKind
	Val  string
}

// keywords riconosciute (case-insensitive). Il valore canonico è maiuscolo.
var keywords = map[string]bool{
	"AND": true, "OR": true, "NOT": true, "IN": true, "IS": true,
	"EMPTY": true, "NULL": true, "ORDER": true, "BY": true, "ASC": true, "DESC": true,
}

// Lex trasforma la stringa JQL in una lista di token terminata da TokEOF.
func Lex(input string) ([]Token, error) {
	var toks []Token
	rs := []rune(input)
	i := 0
	for i < len(rs) {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '(':
			toks = append(toks, Token{Kind: TokLParen, Val: "("})
			i++
		case c == ')':
			toks = append(toks, Token{Kind: TokRParen, Val: ")"})
			i++
		case c == ',':
			toks = append(toks, Token{Kind: TokComma, Val: ","})
			i++
		case c == '"' || c == '\'':
			quote := c
			i++
			start := i
			for i < len(rs) && rs[i] != quote {
				i++
			}
			if i >= len(rs) {
				return nil, fmt.Errorf("stringa non terminata")
			}
			toks = append(toks, Token{Kind: TokString, Val: string(rs[start:i])})
			i++ // salta quote di chiusura
		case c == '=' || c == '~':
			toks = append(toks, Token{Kind: TokOp, Val: string(c)})
			i++
		case c == '!':
			if i+1 < len(rs) && (rs[i+1] == '=' || rs[i+1] == '~') {
				toks = append(toks, Token{Kind: TokOp, Val: string(rs[i : i+2])})
				i += 2
			} else {
				return nil, fmt.Errorf("carattere inatteso '!'")
			}
		case c == '>' || c == '<':
			if i+1 < len(rs) && rs[i+1] == '=' {
				toks = append(toks, Token{Kind: TokOp, Val: string(rs[i : i+2])})
				i += 2
			} else {
				toks = append(toks, Token{Kind: TokOp, Val: string(c)})
				i++
			}
		default:
			// identificatore/numero/keyword: sequenza di caratteri non separatori
			start := i
			for i < len(rs) && !isSep(rs[i]) {
				i++
			}
			word := string(rs[start:i])
			up := strings.ToUpper(word)
			if keywords[up] {
				toks = append(toks, Token{Kind: TokKeyword, Val: up})
			} else if isNumber(word) {
				toks = append(toks, Token{Kind: TokNumber, Val: word})
			} else {
				toks = append(toks, Token{Kind: TokIdent, Val: word})
			}
		}
	}
	toks = append(toks, Token{Kind: TokEOF})
	return toks, nil
}

func isSep(r rune) bool {
	return unicode.IsSpace(r) || r == '(' || r == ')' || r == ',' ||
		r == '=' || r == '!' || r == '~' || r == '>' || r == '<' ||
		r == '"' || r == '\''
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	for i, r := range s {
		if r == '-' && i == 0 {
			continue
		}
		if r == '.' && !dot {
			dot = true
			continue
		}
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
