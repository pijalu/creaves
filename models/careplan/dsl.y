// Matcher DSL grammar for the Care Expert System (§5.2 of
// creaves/docs/care-expert.md). Precedence NOT > AND > OR is structural
// (layered nonterminals); BETWEEN owns its AND (SQL-style); malformed-input
// productions produce the precise, position-tagged error messages the admin
// UI shows inline.
//
// The Go parser is GENERATED — dsl_yacc.go is a build artifact, NOT
// committed (generated code that regenerates easily during the build phase
// stays out of VCS). Regenerate with:
//
//	go generate ./models/careplan
//
// which runs the //go:generate directives in dsl.go (goyacc from
// golang.org/x/tools/cmd/goyacc; install: go install
// golang.org/x/tools/cmd/goyacc@latest). The generated file is
// self-contained (no x/tools runtime import at build time).
//
// The lexer feeding the generated parser (dsl.go) is likewise not
// hand-rolled: it adapts the standard-library text/scanner tokenizer,
// adding only the DSL's two-char operators (<=, >=, !=, !~),
// case-insensitive keywords, negative-number folding and the regex-safe
// unquote (only \" and \\ are escapes, so RE2 patterns like "\d{3}"
// survive verbatim).
//
%{

package careplan

%}

%union {
	node Node
	lit  Literal
	lits []Literal
	op   string
	fp   fieldPos
	pos  Pos
}

%start start

%token <fp>  T_IDENT
%token <lit> T_LSTR T_LNUM T_TRUE T_FALSE
%token <op>  T_EQ T_NEQ T_LT T_LE T_GT T_GE T_TILDE T_NTILDE T_CONTAINS
%token <op>  T_EQCI T_NEQCI
%token <pos> T_AND T_OR T_NOT T_BETWEEN T_IN T_INCI
%token       T_LPAREN T_RPAREN T_COMMA T_EOF

%type <node> start expr_top or_expr and_expr unary predicate
%type <node> cmp_pred between_pred in_pred regex_pred contains_pred
%type <lits> in_items
%type <lit>  literal
%type <fp>   fieldname cmp

%%

start:
	expr_top T_EOF
		{ yylex.(*lexer).ast = $1 }
	| T_EOF
		{ yylex.(*lexer).errorAt($<pos>1, "unexpected end of expression") }
	| error
		{ $$ = nil } // message already recorded by lexer.Error at the offending token
	;

expr_top:
	or_expr
		{ $$ = $1 }
	;

or_expr:
	and_expr
		{ $$ = $1 }
	| or_expr T_OR and_expr
		{ $$ = &OrNode{Left: $1, Right: $3, pos: $<pos>2} }
	| or_expr T_OR T_EOF
		{ yylex.(*lexer).errorAt($<pos>3, "unexpected end of expression") }
	;

and_expr:
	unary
		{ $$ = $1 }
	| and_expr T_AND unary
		{ $$ = &AndNode{Left: $1, Right: $3, pos: $<pos>2} }
	| and_expr T_AND T_EOF
		{ yylex.(*lexer).errorAt($<pos>3, "unexpected end of expression") }
	;

unary:
	predicate
		{ $$ = $1 }
	| T_NOT unary
		{ $$ = &NotNode{Expr: $2, pos: $<pos>1} }
	| T_NOT T_EOF
		{ yylex.(*lexer).errorAt($<pos>2, "unexpected end of expression") }
	| T_LPAREN or_expr T_RPAREN
		{ $$ = $2 }
	| T_LPAREN or_expr T_EOF
		{ yylex.(*lexer).errorAt($<pos>3, "expected )") }
	;

predicate:
	cmp_pred
	| between_pred
	| in_pred
	| regex_pred
	| contains_pred
	;

cmp_pred:
	fieldname cmp literal
		{
			$$ = &PredicateNode{
				Field: $1.name, fieldPos: $1.pos,
				Op: $2.name, opPos: $2.pos,
				Literal: $3,
				pos:  $1.pos,
			}
		}
	| fieldname cmp T_EOF
		{ yylex.(*lexer).errorAt($<pos>3, "unexpected end of expression") }
	;

between_pred:
	fieldname T_BETWEEN T_LNUM T_AND T_LNUM
		{
			$$ = &BetweenNode{
				Field: $1.name, fieldPos: $1.pos, opPos: $<pos>2,
				Min: $3.Num, minPos: $3.Pos,
				Max: $5.Num, maxPos: $5.Pos,
				pos: $1.pos,
			}
		}
	| fieldname T_BETWEEN T_LNUM T_EOF
		{ yylex.(*lexer).errorAt($<pos>4, "expected AND") }
	| fieldname T_BETWEEN T_LNUM T_LSTR
		{ yylex.(*lexer).errorAt($<pos>4, "expected AND") }
	| fieldname T_BETWEEN T_LNUM T_RPAREN
		{ yylex.(*lexer).errorAt($<pos>4, "expected AND") }
	| fieldname T_BETWEEN T_LNUM T_COMMA
		{ yylex.(*lexer).errorAt($<pos>4, "expected AND") }
	| fieldname T_BETWEEN T_LSTR
		{ yylex.(*lexer).errorAt($<pos>3, "BETWEEN expects number bounds") }
	| fieldname T_BETWEEN T_TRUE
		{ yylex.(*lexer).errorAt($<pos>3, "BETWEEN expects number bounds") }
	| fieldname T_BETWEEN T_FALSE
		{ yylex.(*lexer).errorAt($<pos>3, "BETWEEN expects number bounds") }
	| fieldname T_BETWEEN T_EOF
		{ yylex.(*lexer).errorAt($<pos>3, "unexpected end of expression") }
	;

in_pred:
	fieldname T_IN T_LPAREN in_items T_RPAREN
		{
			$$ = &InNode{
				Field: $1.name, fieldPos: $1.pos, opPos: $<pos>2,
				Op:     OpIn,
				Literals: $4,
				pos:   $1.pos,
			}
		}
	| fieldname T_IN T_LPAREN T_RPAREN
		{
			$$ = &InNode{Field: $1.name, fieldPos: $1.pos, opPos: $<pos>2, Op: OpIn, pos: $1.pos}
		}
	| fieldname T_IN T_LPAREN in_items T_EOF
		{ yylex.(*lexer).errorAt($<pos>5, "expected , or )") }
	| fieldname T_INCI T_LPAREN in_items T_RPAREN
		{
			$$ = &InNode{
				Field: $1.name, fieldPos: $1.pos, opPos: $<pos>2,
				Op:     OpInCI,
				Literals: $4,
				pos:   $1.pos,
			}
		}
	| fieldname T_INCI T_LPAREN T_RPAREN
		{
			$$ = &InNode{Field: $1.name, fieldPos: $1.pos, opPos: $<pos>2, Op: OpInCI, pos: $1.pos}
		}
	| fieldname T_INCI T_LPAREN in_items T_EOF
		{ yylex.(*lexer).errorAt($<pos>5, "expected , or )") }
	;

in_items:
	literal
		{ $$ = []Literal{$1} }
	| in_items T_COMMA literal
		{ $$ = append($1, $3) }
	| in_items literal
		{ yylex.(*lexer).errorAt($2.Pos, "expected , or )"); $$ = $1 }
	;

regex_pred:
	fieldname T_TILDE T_LSTR
		{ $$ = yylex.(*lexer).regexPred($1, $<pos>2, OpRegex, $3) }
	| fieldname T_NTILDE T_LSTR
		{ $$ = yylex.(*lexer).regexPred($1, $<pos>2, OpNotRegex, $3) }
	| fieldname T_TILDE T_LNUM
		{ yylex.(*lexer).errorAt($3.Pos, "regex patterns must be double-quoted strings") }
	| fieldname T_NTILDE T_LNUM
		{ yylex.(*lexer).errorAt($3.Pos, "regex patterns must be double-quoted strings") }
	| fieldname T_TILDE T_TRUE
		{ yylex.(*lexer).errorAt($3.Pos, "regex patterns must be double-quoted strings") }
	| fieldname T_NTILDE T_TRUE
		{ yylex.(*lexer).errorAt($3.Pos, "regex patterns must be double-quoted strings") }
	| fieldname T_TILDE T_FALSE
		{ yylex.(*lexer).errorAt($3.Pos, "regex patterns must be double-quoted strings") }
	| fieldname T_NTILDE T_FALSE
		{ yylex.(*lexer).errorAt($3.Pos, "regex patterns must be double-quoted strings") }
	;

contains_pred:
	fieldname T_CONTAINS T_LSTR
		{
			$$ = &PredicateNode{
				Field: $1.name, fieldPos: $1.pos,
				Op: $2, opPos: $<pos>2,
				Literal: $3,
				pos:  $1.pos,
			}
		}
	| fieldname T_CONTAINS T_LNUM
		{ yylex.(*lexer).errorAt($3.Pos, "CONTAINS expects a double-quoted string") }
	| fieldname T_CONTAINS T_TRUE
		{ yylex.(*lexer).errorAt($3.Pos, "CONTAINS expects a double-quoted string") }
	| fieldname T_CONTAINS T_FALSE
		{ yylex.(*lexer).errorAt($3.Pos, "CONTAINS expects a double-quoted string") }
	;

fieldname:
	T_IDENT
		{ $$ = $1 }
	;

cmp:
	T_EQ
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_NEQ
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_EQCI
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_NEQCI
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_LT
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_LE
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_GT
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	| T_GE
		{ $$ = fieldPos{name: $1, pos: $<pos>1} }
	;

literal:
	T_LSTR
		{ $$ = $1 }
	| T_LNUM
		{ $$ = $1 }
	| T_TRUE
		{ $$ = $1 }
	| T_FALSE
		{ $$ = $1 }
	;
