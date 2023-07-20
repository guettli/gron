package gron

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// A Statement is a slice of tokens representing an assignment Statement.
// An assignment Statement is something like:
//
//	json.city = "Leeds";
//
// Where 'json', '.', 'city', '=', '"Leeds"' and ';' are discrete tokens.
// Statements are stored as tokens to make sorting more efficient, and so
// that the same type can easily be used when gronning and ungronning.
type Statement []Token

// String returns the string form of a statement rather than the
// underlying slice of tokens
func (s Statement) String() string {
	out := make([]string, 0, len(s)+2)
	for _, t := range s {
		out = append(out, t.format())
	}
	return strings.Join(out, "")
}

// colorString returns the string form of a statement with ASCII color codes
func (s Statement) colorString() string {
	out := make([]string, 0, len(s)+2)
	for _, t := range s {
		out = append(out, t.formatColor())
	}
	return strings.Join(out, "")
}

// a StatementConv converts a statement to string
type StatementConv func(s Statement) string

// statementconv variant of statement.String
func StatementToString(s Statement) string {
	return s.String()
}

// statementconv variant of statement.colorString
func StatementToColorString(s Statement) string {
	return s.colorString()
}

// withBare returns a copy of a statement with a new bare
// word token appended to it
func (s Statement) withBare(k string) Statement {
	new := make(Statement, len(s), len(s)+2)
	copy(new, s)
	return append(
		new,
		Token{".", TypDot},
		Token{k, TypBare},
	)
}

// Jsonify converts an assignment statement to a JSON representation
func (s Statement) Jsonify() (Statement, error) {
	// If m is the number of keys occurring in the left hand side
	// of s, then len(s) is in between 2*m+4 and 3*m+4. The resultant
	// statement j (carrying the JSON representation) is always 2*m+5
	// long. So len(s)+1 ≥ 2*m+5 = len(j). Therefore an initaial
	// allocation of j with capacity len(s)+1 will allow us to carry
	// through without reallocation.
	j := make(Statement, 0, len(s)+1)
	if len(s) < 4 || s[0].Typ != TypBare || s[len(s)-3].Typ != TypEquals ||
		s[len(s)-1].Typ != TypSemi {
		return nil, errors.New("non-assignment statement")
	}

	j = append(j, Token{"[", TypLBrace})
	j = append(j, Token{"[", TypLBrace})
	for _, t := range s[1 : len(s)-3] {
		switch t.Typ {
		case TypNumericKey, TypQuotedKey:
			j = append(j, t)
			j = append(j, Token{",", TypComma})
		case TypBare:
			j = append(j, Token{quoteString(t.Text), TypQuotedKey})
			j = append(j, Token{",", TypComma})
		}
	}
	if j[len(j)-1].Typ == TypComma {
		j = j[:len(j)-1]
	}
	j = append(j, Token{"]", TypLBrace})
	j = append(j, Token{",", TypComma})
	j = append(j, s[len(s)-2])
	j = append(j, Token{"]", TypLBrace})

	return j, nil
}

// withQuotedKey returns a copy of a statement with a new
// quoted key token appended to it
func (s Statement) withQuotedKey(k string) Statement {
	new := make(Statement, len(s), len(s)+3)
	copy(new, s)
	return append(
		new,
		Token{"[", TypLBrace},
		Token{quoteString(k), TypQuotedKey},
		Token{"]", TypRBrace},
	)
}

// withNumericKey returns a copy of a statement with a new
// numeric key token appended to it
func (s Statement) withNumericKey(k int) Statement {
	new := make(Statement, len(s), len(s)+3)
	copy(new, s)
	return append(
		new,
		Token{"[", TypLBrace},
		Token{strconv.Itoa(k), TypNumericKey},
		Token{"]", TypRBrace},
	)
}

// Statements is a list of assignment Statements.
// E.g statement: json.foo = "bar";
type Statements []Statement

// AddWithValue takes a statement representing a path, copies it,
// adds a value token to the end of the statement and appends
// the new statement to the list of statements
func (ss *Statements) AddWithValue(path Statement, value Token) {
	s := make(Statement, len(path), len(path)+3)
	copy(s, path)
	s = append(s, Token{"=", TypEquals}, value, Token{";", TypSemi})
	*ss = append(*ss, s)
}

// Add appends a new complete statement to list of statements
func (ss *Statements) Add(s Statement) {
	*ss = append(*ss, s)
}

// Len returns the number of statements for sort.Sort
func (ss Statements) Len() int {
	return len(ss)
}

// Swap swaps two statements for sort.Sort
func (ss Statements) Swap(i, j int) {
	ss[i], ss[j] = ss[j], ss[i]
}

// a StatementMaker is a function that makes a statement
// from string
type StatementMaker func(str string) (Statement, error)

// StatementFromString takes statement string, lexes it and returns
// the corresponding statement
func StatementFromString(str string) Statement {
	l := newLexer(str)
	s := l.lex()
	return s
}

// statementmaker variant of statementFromString
func StatementFromStringMaker(str string) (Statement, error) {
	return StatementFromString(str), nil
}

// statementFromJson returns statement encoded by
// JSON specification
func StatementFromJSONSpec(str string) (Statement, error) {
	var a []interface{}
	var ok bool
	var v interface{}
	var s Statement
	var t TokenTyp
	var nstr string
	var nbuf []byte

	err := json.Unmarshal([]byte(str), &a)
	if err != nil {
		return nil, err
	}
	if len(a) != 2 {
		goto out
	}

	v = a[1]
	a, ok = a[0].([]interface{})
	if !ok {
		goto out
	}

	// We'll append one initial token, then 3 tokens for each element of a,
	// then 3 closing tokens, that's altogether 3*len(a)+4.
	s = make(Statement, 0, 3*len(a)+4)
	s = append(s, Token{"json", TypBare})
	for _, e := range a {
		s = append(s, Token{"[", TypLBrace})
		switch e := e.(type) {
		case string:
			s = append(s, Token{quoteString(e), TypQuotedKey})
		case float64:
			nbuf, err = json.Marshal(e)
			if err != nil {
				return nil, errors.Wrap(err, "JSON internal error")
			}
			nstr = fmt.Sprintf("%s", nbuf)
			s = append(s, Token{nstr, TypNumericKey})
		default:
			ok = false
			goto out
		}
		s = append(s, Token{"]", TypRBrace})
	}

	s = append(s, Token{"=", TypEquals})

	switch v := v.(type) {
	case bool:
		if v {
			t = TypTrue
		} else {
			t = TypFalse
		}
	case float64:
		t = TypNumber
	case string:
		t = TypString
	case []interface{}:
		ok = (len(v) == 0)
		if !ok {
			goto out
		}
		t = TypEmptyArray
	case map[string]interface{}:
		ok = (len(v) == 0)
		if !ok {
			goto out
		}
		t = TypEmptyObject
	default:
		ok = (v == nil)
		if !ok {
			goto out
		}
		t = TypNull
	}

	nbuf, err = json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, "JSON internal error")
	}
	nstr = fmt.Sprintf("%s", nbuf)
	s = append(s, Token{nstr, t})

	s = append(s, Token{";", TypSemi})

out:
	if !ok {
		return nil, errors.New("invalid JSON layout")
	}
	return s, nil
}

// ungron turns statements into a proper datastructure
func (ss Statements) ToInterface() (interface{}, error) {

	// Get all the individually parsed statements
	var parsed []interface{}
	for _, s := range ss {
		u, err := ungronTokens(s)

		switch err.(type) {
		case nil:
			// no problem :)
		case errRecoverable:
			continue
		default:
			return nil, errors.Wrapf(err, "ungron failed for `%s`", s)
		}

		parsed = append(parsed, u)
	}

	if len(parsed) == 0 {
		return nil, fmt.Errorf("no statements were parsed")
	}

	merged := parsed[0]
	for _, p := range parsed[1:] {
		m, err := recursiveMerge(merged, p)
		if err != nil {
			return nil, errors.Wrap(err, "failed to merge statements")
		}
		merged = m
	}
	return merged, nil

}

// Less compares two statements for sort.Sort
// Implements a natural sort to keep array indexes in order
func (ss Statements) Less(a, b int) bool {

	// ss[a] and ss[b] are both slices of tokens. The first
	// thing we need to do is find the first token (if any)
	// that differs, then we can use that token to decide
	// if ss[a] or ss[b] should come first in the sort.
	diffIndex := -1
	for i := range ss[a] {

		if len(ss[b]) < i+1 {
			// b must be shorter than a, so it
			// should come first
			return false
		}

		// The tokens match, so just carry on
		if ss[a][i] == ss[b][i] {
			continue
		}

		// We've found a difference
		diffIndex = i
		break
	}

	// If diffIndex is still -1 then the only difference must be
	// that ss[b] is longer than ss[a], so ss[a] should come first
	if diffIndex == -1 {
		return true
	}

	// Get the tokens that differ
	ta := ss[a][diffIndex]
	tb := ss[b][diffIndex]

	// An equals always comes first
	if ta.Typ == TypEquals {
		return true
	}
	if tb.Typ == TypEquals {
		return false
	}

	// If both tokens are numeric keys do an integer comparison
	if ta.Typ == TypNumericKey && tb.Typ == TypNumericKey {
		ia, _ := strconv.Atoi(ta.Text)
		ib, _ := strconv.Atoi(tb.Text)
		return ia < ib
	}

	// If neither token is a number, just do a string comparison
	if ta.Typ != TypNumber || tb.Typ != TypNumber {
		return ta.Text < tb.Text
	}

	// We have two numbers to compare so turn them into json.Number
	// for comparison
	na, _ := json.Number(ta.Text).Float64()
	nb, _ := json.Number(tb.Text).Float64()
	return na < nb

}

// Contains searches the statements for a given statement
// Mostly to make testing things easier
func (ss Statements) Contains(search Statement) bool {
	for _, i := range ss {
		if reflect.DeepEqual(i, search) {
			return true
		}
	}
	return false
}

// StatementsFromJSON takes an io.Reader containing JSON
// and returns statements or an error on failure
func StatementsFromJSON(r Decoder, prefix Statement) (Statements, error) {
	var top interface{}
	err := r.Decode(&top)
	if err != nil {
		return nil, err
	}
	ss := make(Statements, 0, 32)
	ss.fill(prefix, top)
	return ss, nil
}

// fill takes a prefix statement and some value and recursively fills
// the statement list using that value
func (ss *Statements) fill(prefix Statement, v interface{}) {

	// Add a statement for the current prefix and value
	ss.AddWithValue(prefix, valueTokenFromInterface(v))

	// Recurse into objects and arrays
	switch vv := v.(type) {

	case map[interface{}]interface{}:
		// It's an object
		for k, sub := range vv {
			ks := fmt.Sprintf("%v", k)
			if validIdentifier(ks) {
				ss.fill(prefix.withBare(ks), sub)
			} else {
				ss.fill(prefix.withQuotedKey(ks), sub)
			}
		}
	case map[string]interface{}:
		// It's an object
		for k, sub := range vv {
			if validIdentifier(k) {
				ss.fill(prefix.withBare(k), sub)
			} else {
				ss.fill(prefix.withQuotedKey(k), sub)
			}
		}

	case []interface{}:
		// It's an array
		for k, sub := range vv {
			ss.fill(prefix.withNumericKey(k), sub)
		}
	}

}
