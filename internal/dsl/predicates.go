package dsl

import (
	"fmt"
	"go/token"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
)

type Predicate[E any] interface {
	Evaluate(E) bool
	String() string
}

func Not[E any](p Predicate[E]) Predicate[E]        { return not[E]{Predicate: p} }
func And[E any](preds ...Predicate[E]) Predicate[E] { return and[E](preds) }
func Or[E any](preds ...Predicate[E]) Predicate[E]  { return or[E](preds) }

// CallStackIncludes matches when a function name in the current call stack
// contains the provided substring.
func CallStackIncludes[E any](name string) Predicate[E] {
	return callStackIncludes[E](name)
}

type callStackIncludes[E any] string

func (p callStackIncludes[E]) String() string {
	return fmt.Sprintf("(CallStackIncludes %q)", string(p))
}

func (p callStackIncludes[E]) Evaluate(E) bool {
	var pcs [32]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, string(p)) {
			return true
		}
		if !more {
			return false
		}
	}
}
func OnIndex[E any](n int32) *Index[E] {
	p := new(Index[E])
	p.Int32.Store(n)
	return p
}

type Index[E any] struct{ atomic.Int32 }

func (p *Index[E]) String() string  { return fmt.Sprintf("(OnIndex %d)", p.Int32.Load()) }
func (p *Index[E]) Evaluate(E) bool { return p.Int32.Add(-1) == -1 }

type not[E any] struct{ Predicate[E] }

func (p not[E]) String() string    { return fmt.Sprintf("(Not %s)", p.Predicate.String()) }
func (p not[E]) Evaluate(e E) bool { return !p.Predicate.Evaluate(e) }

type and[E any] []Predicate[E]

func (p and[E]) String() string {
	var b strings.Builder
	b.WriteString("(And")
	for _, pred := range p {
		fmt.Fprintf(&b, " %s", pred.String())
	}
	b.WriteByte(')')
	return b.String()
}
func (p and[E]) Evaluate(e E) bool {
	for _, pred := range p {
		if !pred.Evaluate(e) {
			return false
		}
	}
	return true
}

type or[E any] []Predicate[E]

func (p or[E]) String() string {
	var b strings.Builder
	b.WriteString("(Or")
	for _, pred := range p {
		fmt.Fprintf(&b, " %s", pred.String())
	}
	b.WriteByte(')')
	return b.String()
}
func (p or[E]) Evaluate(e E) bool {
	for _, pred := range p {
		if pred.Evaluate(e) {
			return true
		}
	}
	return false
}

func parseNot[E any](p *Parser[Predicate[E]], s *Scanner) Predicate[E] {
	preds := parseVariadicPredicate(p, s)
	if len(preds) != 1 {
		panic(fmt.Errorf("dsl: not accepts exactly 1 argument, given %d", len(preds)))
	}
	return Not(preds[0])
}

func parseAnd[E any](p *Parser[Predicate[E]], s *Scanner) Predicate[E] {
	return And(parseVariadicPredicate(p, s)...)
}
func parseOr[E any](p *Parser[Predicate[E]], s *Scanner) Predicate[E] {
	return Or(parseVariadicPredicate(p, s)...)
}
func parseOnIndex[E any](_ *Parser[Predicate[E]], s *Scanner) Predicate[E] {
	i, err := strconv.ParseInt(s.Consume(token.INT).Lit, 10, 32)
	if err != nil {
		panic(err)
	}
	s.Consume(token.RPAREN)
	return OnIndex[E](int32(i))
}

func parseCallStackIncludes[E any](_ *Parser[Predicate[E]], s *Scanner) Predicate[E] {
	name := s.ConsumeString()
	s.Consume(token.RPAREN)
	return CallStackIncludes[E](name)
}

func parseVariadicPredicate[E any](p *Parser[Predicate[E]], s *Scanner) (ret []Predicate[E]) {
	tok := s.Scan()
	for tok.Kind == token.LPAREN || tok.Kind == token.IDENT {
		ret = append(ret, p.ParseFromPos(s, tok))
		tok = s.Scan()
	}
	assertTok(tok, token.RPAREN)
	return ret
}
