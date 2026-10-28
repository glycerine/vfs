package dsl

import (
	"fmt"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

func NewParser[T any]() *Parser[T] {
	return &Parser[T]{
		constants: make(map[string]func() T),
		funcs:     make(map[string]func(*Parser[T], *Scanner) T),
	}
}

func NewPredicateParser[E any]() *Parser[Predicate[E]] {
	p := NewParser[Predicate[E]]()
	p.DefineFunc("Not", parseNot[E])
	p.DefineFunc("And", parseAnd[E])
	p.DefineFunc("Or", parseOr[E])
	p.DefineFunc("OnIndex", parseOnIndex[E])
	p.DefineFunc("CallStackIncludes", parseCallStackIncludes[E])
	return p
}

type Parser[T any] struct {
	constants map[string]func() T
	funcs     map[string]func(*Parser[T], *Scanner) T
}

func (p *Parser[T]) DefineConstant(identifier string, instantiate func() T) {
	p.constants[identifier] = instantiate
}

func (p *Parser[T]) DefineFunc(identifier string, parse func(*Parser[T], *Scanner) T) {
	p.funcs[identifier] = parse
}

func (p *Parser[T]) Parse(input string) (ret T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			var ok bool
			if err, ok = recovered.(error); !ok {
				panic(recovered)
			}
		}
	}()
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(input))
	var s Scanner
	s.Init(file, []byte(strings.TrimSpace(input)), nil, 0)
	ret = p.ParseFromPos(&s, s.Scan())
	tok := s.Scan()
	if tok.Kind == token.SEMICOLON {
		tok = s.Scan()
	}
	assertTok(tok, token.EOF)
	return ret, err
}

func (p *Parser[T]) ParseFromPos(s *Scanner, tok Token) T {
	switch tok.Kind {
	case token.IDENT:
		instantiate, ok := p.constants[tok.Lit]
		if !ok {
			panic(fmt.Errorf("dsl: unknown constant %q", tok.Lit))
		}
		return instantiate()
	case token.LPAREN:
		tok = s.Consume(token.IDENT)
		parse, ok := p.funcs[tok.Lit]
		if !ok {
			panic(fmt.Errorf("dsl: unknown func %q", tok.Lit))
		}
		return parse(p, s)
	default:
		panic(fmt.Errorf("dsl: unexpected token %s; expected IDENT or LPAREN", tok.String()))
	}
}

type Scanner struct{ scanner.Scanner }

func (s *Scanner) Scan() Token {
	pos, kind, lit := s.Scanner.Scan()
	return Token{pos: pos, Kind: kind, Lit: lit}
}

func (s *Scanner) Consume(expect token.Token) Token {
	tok := s.Scan()
	assertTok(tok, expect)
	return tok
}

func (s *Scanner) ConsumeString() string {
	lit := s.Consume(token.STRING).Lit
	value, err := strconv.Unquote(lit)
	if err != nil {
		panic(fmt.Errorf("dsl: unquoting %q: %v", lit, err))
	}
	return value
}

type Token struct {
	pos  token.Pos
	Kind token.Token
	Lit  string
}

func (t *Token) String() string {
	if t.Lit != "" {
		return fmt.Sprintf("(%s, %q) at pos %v", t.Kind, t.Lit, t.pos)
	}
	return fmt.Sprintf("%s at pos %v", t.Kind, t.pos)
}

func assertTok(tok Token, expect token.Token) {
	if tok.Kind != expect {
		panic(fmt.Errorf("dsl: unexpected token %s; expected %s", tok.String(), expect))
	}
}
