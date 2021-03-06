package main

import (
	"errors"
	"strconv"
	"strings"
)

const MAX_ARGS = 6
const MAX_OP_PRIO = 16
const MAX_ALIGN = 16

var gstrings []*Ast
var flonums []*Ast
var globalenv = &Dict{}
var localenv *Dict
var struct_defs Dict
var union_defs Dict
var typedefs Dict
var localvars []*Ast
var current_func_type *Ctype
var labelseq = 0

var ctype_void = &Ctype{typ: CTYPE_VOID, size: 0, sig: true,}
var ctype_char = &Ctype{typ: CTYPE_CHAR, size: 1, sig: true,}
var ctype_short = &Ctype{typ: CTYPE_SHORT, size: 2, sig: true}
var ctype_int = &Ctype{typ: CTYPE_INT, size: 4, sig: true,}
var ctype_long = &Ctype{typ: CTYPE_LONG, size: 8, sig: true,}
var ctype_float = &Ctype{typ: CTYPE_FLOAT, size: 4, sig: true,}
var ctype_double = &Ctype{typ: CTYPE_DOUBLE, size: 8, sig: true,}

var ctype_ulong = &Ctype{typ: CTYPE_LONG, size: 8, sig: false,}

const (
	S_TYPEDEF int = iota + 1
	S_EXTERN
	S_STATIC
	S_AUTO
	S_REGISTER
)

const (
	DECL_BODY int = iota + 1
	DECL_PARAM
	DECL_PARAM_TYPEONLY
	DECL_CAST
)

func ast_uop(typ int, ctype *Ctype, operand *Ast) *Ast {
	r := &Ast{}
	r.typ = typ
	r.ctype = ctype
	r.operand = operand
	return r
}

func ast_binop(typ int, left *Ast, right *Ast) *Ast {
	r := &Ast{}
	r.typ = typ
	r.ctype = result_type(byte(typ), left.ctype, right.ctype)
	if typ != '=' && convert_array(left.ctype).typ != CTYPE_PTR &&
		convert_array(right.ctype).typ == CTYPE_PTR {
		r.left = right
		r.right = left
	} else {
		r.left = left
		r.right = right
	}
	return r
}

func ast_inttype(ctype *Ctype, val int) *Ast {
	r := &Ast{}
	r.typ = AST_LITERAL
	r.ctype = ctype
	r.ival = val
	return r
}

func ast_double(val float64) *Ast {
	r := &Ast{}
	r.typ = AST_LITERAL
	r.ctype = ctype_double
	r.fval = val
	flonums = append(flonums, r)
	return r
}

func make_label() string {
	s := format(".L%d", labelseq)
	labelseq++
	return s
}

func ast_lvar(ctype *Ctype, name string) *Ast {
	r := &Ast{}
	r.typ = AST_LVAR
	r.ctype = ctype
	r.varname = name
	localenv.PutAst(name, r)
	if localvars != nil {
		localvars = append(localvars, r)
	}

	return r
}

type MakeVarFn func(ctype *Ctype, name string) *Ast

func define_struct_union_field(opaque *Dict, ctype *Ctype, name string) {
	opaque.PutCtype(name, ctype)
}

func ast_gvar(ctype *Ctype, name string) *Ast {
	r := &Ast{}
	r.typ = AST_GVAR
	r.ctype = ctype
	r.varname = name
	r.glabel = name
	globalenv.PutAst(name, r)
	return r
}

func ast_string(str string) *Ast {
	r := &Ast{}
	r.typ = AST_STRING
	r.ctype = make_array_type(ctype_char, len(str)+1)
	r.val = str
	r.slabel = make_label()
	return r
}

func ast_funcall(ctype *Ctype, fname string, args []*Ast, paramtypes []*Ctype) *Ast {
	r := &Ast{}
	r.typ = AST_FUNCALL
	r.ctype = ctype
	r.fname = fname
	r.args = args
	r.paramtypes = paramtypes
	return r
}

func ast_func(rettype *Ctype, fname string, params []*Ast, localvars []*Ast, body *Ast) *Ast {
	r := &Ast{}
	r.typ = AST_FUNC
	r.ctype = rettype
	r.fname = fname
	r.params = params
	r.localvars = localvars
	r.body = body
	return r
}

func ast_decl(variable *Ast, init *Ast) *Ast {
	r := &Ast{}
	r.typ = AST_DECL
	r.ctype = nil
	r.declvar = variable
	r.declinit = init
	return r
}

func ast_init_list(initlist []*Ast) *Ast {
	r := &Ast{}
	r.typ = AST_INIT_LIST
	r.ctype = nil
	r.initlist = initlist
	return r
}

func ast_if(cond *Ast, then *Ast, els *Ast) *Ast {
	r := &Ast{}
	r.typ = AST_IF
	r.ctype = nil
	r.cond = cond
	r.then = then
	r.els = els
	return r
}

func ast_ternary(ctype *Ctype, cond *Ast, then *Ast, els *Ast) *Ast {
	r := &Ast{}
	r.typ = AST_TERNARY
	r.ctype = ctype
	r.cond = cond
	r.then = then
	r.els = els
	return r
}

func ast_for(init *Ast, cond *Ast, step *Ast, body *Ast) *Ast {
	r := &Ast{}
	r.typ = AST_FOR
	r.ctype = nil
	r.init = init
	r.cond = cond
	r.step = step
	r.body = body
	return r
}

func ast_return(rettype *Ctype, retval *Ast) *Ast {
	r := &Ast{}
	r.typ = AST_RETURN
	r.ctype = rettype
	r.retval = retval
	return r
}

func ast_compound_stmt(stmts []*Ast) *Ast {
	r := &Ast{}
	r.typ = AST_COMPOUND_STMT
	r.ctype = nil
	r.stmts = stmts
	return r
}

func ast_struct_ref(ctype *Ctype, struc *Ast, name string) *Ast {
	r := &Ast{}
	r.typ = AST_STRUCT_REF
	r.ctype = ctype
	r.struc = struc
	r.field = name
	return r
}

func copy_type(ctype *Ctype) *Ctype {
	copy := *ctype
	return &copy
}

func make_type(typ int, sig bool) *Ctype {
	r := &Ctype{
		typ:typ,
		sig:sig,
	}
	switch typ {
	case CTYPE_VOID:
		r.size = 0
	case CTYPE_CHAR:
		r.size = 1
	case CTYPE_SHORT:
		r.size = 2
	case CTYPE_INT:
		r.size = 4
	case CTYPE_LONG:
		r.size = 8
	case CTYPE_LLONG:
		r.size = 8
	case CTYPE_FLOAT:
		r.size = 8
	case CTYPE_DOUBLE:
		r.size = 8
	case CTYPE_LDOUBLE:
		r.size = 8
	default:
		errorf("internal error")
	}
	return r
}

func make_ptr_type(ctype *Ctype) *Ctype {
	r := &Ctype{}
	r.typ = CTYPE_PTR
	r.ptr = ctype
	r.size = 8
	return r
}

func make_array_type(ctype *Ctype, len int) *Ctype {
	r := &Ctype{}
	r.typ = CTYPE_ARRAY
	r.ptr = ctype
	if len < 0 {
		r.size = -1
	} else {
		r.size = r.ptr.size * len
	}
	r.len = len
	return r
}

func make_struct_field_type(ctype *Ctype, offset int) *Ctype {
	r := copy_type(ctype)
	//r.name = name
	r.offset = offset
	return r
}

func make_struct_type(fields *Dict, size int) *Ctype {
	r := &Ctype{}
	r.typ = CTYPE_STRUCT
	r.fields = fields
	r.size = size
	return r
}

func make_func_type(rettype *Ctype, paramtypes []*Ctype, has_vaargs bool) *Ctype {
	r := &Ctype{}
	r.typ = CTYPE_FUNC
	r.rettype = rettype
	r.params = paramtypes
	r.hasva = has_vaargs
	return r
}

func make_stub_type() *Ctype {
	r := &Ctype{}
	r.typ = CTYPE_STUB
	r.size = 0
	return r
}

func is_inttype(ctype *Ctype) bool {
	return ctype.typ == CTYPE_CHAR || ctype.typ == CTYPE_SHORT ||
		ctype.typ == CTYPE_INT || ctype.typ == CTYPE_LONG || ctype.typ == CTYPE_LLONG
}

func is_flotype(ctype *Ctype) bool {
	return ctype.typ == CTYPE_FLOAT || ctype.typ == CTYPE_DOUBLE ||
		ctype.typ == CTYPE_LDOUBLE
}

func ensure_lvalue(ast *Ast) {
	switch ast.typ {
	case AST_LVAR, AST_GVAR, AST_DEREF, AST_STRUCT_REF:
		return
	}
	errorf("lvalue expected, but got %s", ast)
	return
}

func expect(punct byte) {
	tok := read_token()
	if !tok.is_punct(int(punct)) {
		errorf("'%c' expected but got %s", punct, tok)
	}
}

func (tok *Token) is_ident(s string) bool {
	return tok.is_ident_type() && tok.sval == s
}

func is_right_assoc(tok *Token) bool {
	return tok.punct == '='
}


func E(ast *Ast) int {
	return eval_intexpr(ast)
}

func eval_intexpr(ast *Ast) int {
	L := ast.left
	R := ast.right

	switch ast.typ {
	case AST_LITERAL:
		if is_inttype(ast.ctype) {
			return ast.ival
		}
		errorf("Integer expression expected, but got %s", ast)
	case '!':
		return bool2int(!int2bool(E(ast.operand)))
	case AST_TERNARY:
		if int2bool(E(ast.cond)) {
			return E(ast.then)
		} else {
			return E(ast.els)
		}
	case '+': return E(L) + E(R)
	case '-': return E(L) - E(R)
	case '*': return E(L) * E(R)
	case '/': return E(L) / E(R)
	case '<': return bool2int(E(L) < E(R))
	case '>': return bool2int(E(L) > E(R))
	case OP_EQ: return bool2int(E(L) == E(R))
	case OP_GE: return bool2int(E(L) >= E(R))
	case OP_LE: return bool2int(E(L) <= E(R))
	case OP_NE: return bool2int(E(L) != E(R))
	case OP_LOGAND: return E(L) * E(R)
	case OP_LOGOR: return bool2int(int2bool(E(L)) || int2bool(E(R)))
	default:
		errorf("Integer expression expected, but got %s", ast)
	}
	return -1
}

func priority(tok *Token) int {
	switch tok.punct {
	case '[', '.', OP_ARROW:
		return 1
	case OP_INC, OP_DEC:
		return 2
	case '*', '/':
		return 3
	case '+', '-':
		return 4
	case '<', '>', OP_LE, OP_GE, OP_NE:
		return 6
	case '&':
		return 8
	case '|':
		return 9
	case OP_EQ:
		return 7
	case OP_LOGAND:
		return 11
	case OP_LOGOR:
		return 12
	case '?':
		return 13
	case '=':
		return 14
	default:
		return -1
	}
}

func param_types(params []*Ast) []*Ctype {
	var r []*Ctype
	for _, ast := range params {
		r = append(r, ast.ctype)
	}
	return r
}

func function_type_check(fname string, params []*Ctype, args []*Ctype) {
	if len(args) < len(params) {
		errorf("Too few arguments: %s", fname)
	}
	for i, arg := range args {
		if i < len(params) {
			param := params[i]
			result_type('=', param, arg)
		} else {
			result_type('=', arg, ctype_int)
		}
	}
}

func read_func_args(fname string) *Ast {
	var args []*Ast
	for {
		tok := read_token()
		if tok.is_punct(')') {
			break
		}
		unget_token(tok)
		args = append(args, read_expr())
		tok = read_token()
		if tok.is_punct(')') {
			break
		}
		if !tok.is_punct(',') {
			errorf("Unexpected token: '%s'", tok)
		}
	}
	if MAX_ARGS < len(args) {
		errorf("Too many arguments: %s", fname)
	}
	fnc := localenv.GetAst(fname)
	if fnc != nil {
		t := fnc.ctype
		if t.typ != CTYPE_FUNC {
			errorf("%s is not a function, but %s", fname, t)
		}
		function_type_check(fname, t.params, param_types(args))
		return ast_funcall(t.rettype, fname, args, t.params)
	}
	return ast_funcall(ctype_int, fname, args, nil)
}

func read_ident_or_func(name string) *Ast {
	ch := read_token()
	if ch.is_punct('(') {
		return read_func_args(name)
	}
	unget_token(ch)

	v := localenv.GetAst(name)
	if v == nil {
		errorf("Undefined varaible: %s", name)
	}
	return v
}

func is_long_token(s string) bool {
	for i, c := range []byte(s) {
		if !isdigit(c) {
			return (c == 'L' || c == 'l') && (i == len(s)-1)
		}
	}
	return false
}

func atol(sval string) int {
	s := strings.TrimSuffix(sval, "L")
	i, _ := strconv.Atoi(s)
	return i
}

func read_number_ast(sval string) *Ast {
	assert(sval[0] > 0)
	index := 0
	base := 10
	if sval[0] == '0' {
		index++
		if index < len(sval) && (sval[index] == 'x' || sval[index] == 'X') {
			base = 16
			index++
		} else if index < len(sval) && isdigit(sval[index]) {
			base = 8
		}
	}
	start := index
	for index < len(sval) && (isdigit(sval[index]) ||
		/* 'a' <= && 'f' <= looks to be a bug !! */
		(base == 16 && (('a' <=  sval[index] && 'f' <= sval[index])|| 'A' <= sval[index] && 'F' <= sval[index]) )) {
		index++
	}
	if index < len(sval) && sval[index] == '.' {
		if base != 10 {
			errorf("malformed number: %s", sval)
		}
		index++
		for index < len(sval) && isdigit(sval[index]) {
			index++
		}
		if index < len(sval) && sval[index] != byte(0) {
			errorf("malformed number: %s", sval)
		}
		end := index - 1
		assert(start != end)
		fval, _ := strconv.ParseFloat(sval, 64)
		return ast_double(fval)
	}
	if index < len(sval) && (sval[index] == 'l' || sval[index] == 'L') {
		ival := atol(sval)
		return ast_inttype(ctype_long, ival)
	} else if  index < len(sval) && (sval[index:index+1] == "ul" || sval[index:index+1] == "ul") {
		val, _ := strconv.ParseInt(sval, base, 0)
		return ast_inttype(ctype_long, int(val))
	} else {
		if index < len(sval) && sval[index] != byte(0) {
			errorf("malformed number: %s", sval)
		}
		val, _ := strconv.ParseInt(sval, 0, 64)
		if val >= UINT_MAX {
			return ast_inttype(ctype_long, int(val))
		}
		return ast_inttype(ctype_int, int(val))
	}
}

func read_prim() *Ast {
	tok := read_token()
	if tok == nil {
		return nil
	}
	switch tok.typ {
	case TTYPE_IDENT:
		return read_ident_or_func(tok.sval)
	case TTYPE_NUMBER:
		return read_number_ast(tok.sval)
	case TTYPE_CHAR:
		return ast_inttype(ctype_char, int(tok.c))
	case TTYPE_STRING:
		r := ast_string(tok.sval)
		gstrings = append(gstrings, r)
		return r
	case TTYPE_PUNCT:
		unget_token(tok)
		return nil
	default:
		errorf("Don't know how to handle '%d'", tok.typ)
	}

	return nil
}

func result_type_int(op byte, a *Ctype, b *Ctype) (*Ctype, error) {
	if a.typ > b.typ {
		b, a = a, b
	}

	default_err := errors.New("")
	if b.typ == CTYPE_PTR {
		if op == '=' {
			return a, nil
		}
		if op != '+' && op != '-' {
			return nil, default_err
		}
		if !is_inttype(a) {
			return nil, default_err
		}
		return b, nil
	}

	switch a.typ {
	case CTYPE_VOID:
		return nil, default_err
	case CTYPE_CHAR, CTYPE_SHORT, CTYPE_INT:
		switch b.typ {
		case CTYPE_CHAR, CTYPE_SHORT, CTYPE_INT:
			return ctype_int, nil
		case CTYPE_LONG, CTYPE_LLONG:
			return ctype_long, nil
		case CTYPE_FLOAT, CTYPE_DOUBLE, CTYPE_LDOUBLE:
			return ctype_double, nil
		case CTYPE_ARRAY, CTYPE_PTR:
			return b, nil
		}
		errorf("internal error")
	case CTYPE_LONG, CTYPE_LLONG:
		switch b.typ {
		case CTYPE_LONG, CTYPE_LLONG:
			return ctype_long, nil
		case CTYPE_FLOAT, CTYPE_DOUBLE, CTYPE_LDOUBLE:
			return ctype_double, nil
		case CTYPE_ARRAY, CTYPE_PTR:
			return b, nil
		}
		errorf("internal error")
	case CTYPE_FLOAT:
		if b.typ == CTYPE_FLOAT || b.typ == CTYPE_DOUBLE || b.typ == CTYPE_LDOUBLE {
			return ctype_double, nil
		}
		return nil, default_err
	case CTYPE_DOUBLE, CTYPE_LDOUBLE:
		if b.typ == CTYPE_DOUBLE || b.typ == CTYPE_LDOUBLE  {
			return ctype_double, nil
		}
	case CTYPE_ARRAY:
		if b.typ != CTYPE_ARRAY {
			return nil, default_err
		}

		return result_type_int(op, a.ptr, b.ptr)
	default:
		errorf("internal error: %s %s", a, b)
	}

	return nil, default_err
}

func read_subscript_expr(ast *Ast) *Ast {
	sub := read_expr()
	expect(']')
	t := ast_binop('+', ast, sub)
	return ast_uop(AST_DEREF, t.ctype.ptr, t)
}

func convert_array(ctype *Ctype) *Ctype {
	if ctype.typ != CTYPE_ARRAY {
		return ctype
	}
	return make_ptr_type(ctype.ptr)
}

func result_type(op byte, a *Ctype, b *Ctype) *Ctype {
	ret, err := result_type_int(op, convert_array(a), convert_array(b))
	if err != nil {
		errorf("incompatible operands: %c: <%s> and <%s>",
			op, a, b)
	}
	return ret
}

func get_sizeof_size(allow_typename bool) *Ast {
	tok := read_token()
	if allow_typename && is_type_keyword(tok) {
		unget_token(tok)
		var ctype *Ctype
		read_func_param(&ctype, nil, true)
		return ast_inttype(ctype_long, ctype.size)
	}
	if tok.is_punct('(') {
		r := get_sizeof_size(true)
		expect(')')
		return r
	}
	unget_token(tok)
	expr := read_unary_expr()
	if expr.ctype.size == 0 {
		errorf("invalid operand for sizeof(): %s type=%s size=%d", expr, expr.ctype, expr.ctype.size)
	}
	return ast_inttype(ctype_long, expr.ctype.size)
}

func read_unary_expr() *Ast {
	tok := read_token()
	if tok == nil {
		errorf("premature end of input")
	}
	if tok.is_ident("sizeof") {
		return get_sizeof_size(false)
	}
	if tok.typ != TTYPE_PUNCT {
		unget_token(tok)
		return read_prim()
	}
	if tok.is_punct('(') {
		r := read_expr()
		expect(')')
		return r
	}
	if tok.is_punct('&') {
		operand := read_unary_expr()
		ensure_lvalue(operand)
		return ast_uop(AST_ADDR, make_ptr_type(operand.ctype), operand)
	}
	if tok.is_punct('-') {
		expr := read_expr()
		return ast_binop('-', ast_inttype(ctype_int, 0), expr)
	}
	if tok.is_punct('*') {
		operand := read_unary_expr()
		ctype := convert_array(operand.ctype) // looks no need to call convert_array.
		if ctype.typ != CTYPE_PTR {
			errorf("pointer type expected, but got %", ctype)
		}
		return ast_uop(AST_DEREF, operand.ctype.ptr, operand)
	}
	if tok.is_punct('!') {
		operand := read_unary_expr()
		return ast_uop(int('!'), ctype_int, operand)
	}
	unget_token(tok)
	return read_prim()
}

func read_cond_expr(cond *Ast) *Ast {
	then := read_expr()
	expect(':')
	els := read_expr()
	return ast_ternary(then.ctype, cond, then, els)
}

func read_struct_field(struc *Ast) *Ast {
	if struc.ctype.typ != CTYPE_STRUCT {
		errorf("struct expected, but got %s", struc)
	}
	name := read_token()
	if !name.is_ident_type() {
		errorf("field name expected, but got %s", name)
	}
	field := struc.ctype.fields.GetCtype(name.sval)
	return ast_struct_ref(field, struc, name.sval)
}

func read_expr_int(prec int) *Ast {
	ast := read_unary_expr()
	if ast == nil {
		return nil
	}
	for {
		tok := read_token()
		if tok == nil {
			return ast
		}
		if tok.typ != TTYPE_PUNCT {
			unget_token(tok)
			return ast
		}
		prec2 := priority(tok)
		if prec2 < 0 || prec <= prec2 {
			unget_token(tok)
			return ast
		}

		if tok.is_punct('?') {
			ast = read_cond_expr(ast)
			continue
		}
		if tok.is_punct('.') {
			ast = read_struct_field(ast)
			continue
		}
		if tok.is_punct(OP_ARROW) {
			if ast.ctype.typ != CTYPE_PTR {
				errorf("pointer type expected, but got %s %s",
					ast.ctype, ast)
			}
			ast = ast_uop(AST_DEREF, ast.ctype.ptr, ast)
			ast = read_struct_field(ast)
			continue
		}
		if tok.is_punct('[') {
			ast = read_subscript_expr(ast)
			continue
		}
		// This is BUG?
		if tok.is_punct(OP_INC) || tok.is_punct(OP_DEC) {
			ensure_lvalue(ast)
			ast = ast_uop(tok.punct, ast.ctype, ast)
			continue
		}
		if tok.is_punct('=') {
			ensure_lvalue(ast)
		}
		var prec_incr int
		if is_right_assoc(tok) {
			prec_incr = 1
		} else {
			prec_incr = 0
		}
		rest := read_expr_int(prec2 + prec_incr)
		if rest == nil {
			errorf("second operand missing")
		}
		ast = ast_binop(tok.punct, ast, rest)

	}
	return ast
}

func read_expr() *Ast {
	return read_expr_int(MAX_OP_PRIO)
}

func is_type_keyword(tok *Token) bool {
	if !tok.is_ident_type() {
		return false
	}

	keyword := []string{
		"char", "short", "int", "long", "float", "double", "struct",
		"union", "signed", "unsigned", "enum", "void", "typedef", "extern",
		"static", "auto", "register", "const", "volatile", "inline",
	}
	for _, k := range keyword {
		if k == tok.sval {
			return true
		}
	}

	return typedefs.GetCtype(tok.sval) != nil
}

func read_decl_init_elem(initlist []*Ast, ctype *Ctype) []*Ast {
	tok := peek_token()
	init := read_expr()
	if init == nil {
		errorf("expression expected, but got %s", tok)
	}
	initlist = append(initlist, init)
	result_type('=', init.ctype, ctype)
	init.totype = ctype
	tok = read_token()
	if !tok.is_punct(',') {
		unget_token(tok)
	}
	return initlist
}

func read_decl_array_init_int(initlist []*Ast, ctype *Ctype) []*Ast {
	tok := read_token()
	assert(ctype.typ == CTYPE_ARRAY)
	if ctype.ptr.typ == CTYPE_CHAR && tok.typ == TTYPE_STRING {
		for _,p := range tok.sval {
			c := ast_inttype(ctype_char, int(p))
			c.totype = ctype_char
			initlist = append(initlist, c)
		}
		c := ast_inttype(ctype_char, 0)
		c.totype = ctype_char
		initlist = append(initlist, c)
		return initlist
	}

	if !tok.is_punct('{') {
		errorf("Expected an initializer list, but got %s", tok)
	}
	for {
		tok := read_token()
		if tok.is_punct('}') {
			break
		}
		unget_token(tok)
		initlist = read_decl_init_elem(initlist, ctype.ptr)
	}

	return initlist
}

func read_struct_union_tag() string {
	tok := read_token()
	if tok.is_ident_type() {
		return tok.sval
	} else {
		unget_token(tok)
		return ""
	}
}

func read_struct_union_fields() *Dict {
	tok := read_token()
	if !tok.is_punct('{') {
		unget_token(tok)
		return nil
	}
	r := MakeDict(nil)
	for {
		if !is_type_keyword(peek_token()) {
			break
		}
		basetype, _ := read_decl_spec()
		for {
			var name string
			fieldtype,_ := read_declarator(&name, basetype, nil, DECL_PARAM)
			r.PutCtype(name, fieldtype)
			tok = read_token()
			if tok.is_punct(',') {
				continue
			}
			unget_token(tok)
			expect(';')
			break
		}
	}
	expect('}')
	return r
}

func compute_union_size(fields *Dict) int {
	maxsize := 0
	for _, v := range fields.Values() {
		fieldtype := v.ctype
		if maxsize < fieldtype.size {
			maxsize = fieldtype.size
		}
	}
	return maxsize
}

func compute_struct_size(fields *Dict) int {
	offset := 0
	for _, v := range fields.Values() {
		fieldtype := v.ctype
		var align int
		if fieldtype.size < MAX_ALIGN {
			align = fieldtype.size
		} else {
			align = MAX_ALIGN
		}
		if offset%align != 0 {
			offset += align - offset%align
		}
		fieldtype.offset = offset
		offset += fieldtype.size
	}
	return offset
}

func read_struct_union_def(env *Dict, compute_size func(*Dict)int) *Ctype {
	tag := read_struct_union_tag()
	var prev *Ctype
	if tag != "" {
		prev = env.GetCtype(tag)
	} else {
		prev = nil
	}
	fields := read_struct_union_fields()
	if prev != nil {
		return prev
	}
	var r *Ctype
	if fields != nil {
		r = make_struct_type(fields, compute_size(fields))
	} else {
		r = make_struct_type(nil, 0)
	}
	if tag != "" {
		env.PutCtype(tag, r)
	}
	return r
}

func read_struct_def() *Ctype {
	return read_struct_union_def(&struct_defs, compute_struct_size)
}

func read_union_def() *Ctype {
	return read_struct_union_def(&union_defs, compute_union_size)
}

func read_enum_def() *Ctype {
	tok := read_token()
	if tok.is_ident_type() {
		tok = read_token()
	}
	if !tok.is_punct('{') {
		unget_token(tok)
		return ctype_int
	}
	val := 0
	for {
		tok = read_token()
		if tok.is_punct('}') {
			break
		}
		if !tok.is_ident_type() {
			errorf("Identifier expected, but got %s", tok)
		}
		name := tok.sval

		tok = read_token()
		if tok.is_punct('=') {
			val = eval_intexpr(read_expr())
		} else {
			unget_token(tok)
		}

		constval := ast_inttype(ctype_int, val)
		val++
		if localenv != nil {
			localenv.PutAst(name, constval)
		} else {
			globalenv.PutAst(name, constval)
		}
		tok = read_token()
		if tok.is_punct(',') {
			continue
		}
		if tok.is_punct('}') {
			break
		}
		errorf("',' or '} expected, but got %s", tok)
	}
	return ctype_int
}

func read_direct_declarator2(basetype *Ctype, params []*Ast) (*Ctype, []*Ast) {
	tok := read_token()
	if tok.is_punct('[') {
		var length int
		tok = read_token()
		if tok.is_punct(']') {
			length = -1
		} else {
			unget_token(tok)
			length = eval_intexpr(read_expr())
			expect(']')
		}
		t, params := read_direct_declarator2(basetype, params)
		if t.typ == CTYPE_FUNC {
			errorf("array of functions")
		}
		return make_array_type(t, length), params
	}
	if tok.is_punct('(') {
		if basetype.typ == CTYPE_FUNC {
			errorf("function returning an function")
		}
		if basetype.typ == CTYPE_ARRAY {
			errorf("function returning an array")
		}
		basetype, params = read_func_param_list(basetype, params)
		return basetype, params
	}

	unget_token(tok)
	return basetype, params
}

func skip_type_qualifiers() {
	for {
		tok := read_token()
		if tok.is_ident("const") || tok.is_ident("volatiles") {
			continue
		}
		unget_token(tok)
		return
	}
}

func read_direct_declarator1(rname *string, basetype *Ctype, params []*Ast, ctx int) (*Ctype, []*Ast) {
	tok := read_token()
	next := peek_token()
	if tok.is_punct('(') && !is_type_keyword(next) && !next.is_punct(')') {
		stub := make_stub_type()
		t, params := read_direct_declarator1(rname, stub, params, ctx)
		expect(')')
		ctype, params := read_direct_declarator2(basetype, params)
		*stub = *ctype
		return t, params
	}
	if tok.is_punct('*') {
		skip_type_qualifiers()
		stub := make_stub_type()
		t, params := read_direct_declarator1(rname, stub, params, ctx)
		*stub = *make_ptr_type(basetype)
		return t, params
	}

	if tok.is_ident_type() {
		if ctx == DECL_CAST {
			errorf("identifier is NOT expected, but got %s", tok)
		}
		*rname = tok.sval
		return read_direct_declarator2(basetype, params)
	}
	if ctx == DECL_BODY || ctx == DECL_PARAM {
		errorf("identifier, ( or * are expected, but got %s", tok)
	}
	unget_token(tok)

	return read_direct_declarator2(basetype, params)
}

func fix_array_size(t *Ctype) {
	assert(t.typ != CTYPE_STUB)
	if t.typ == CTYPE_ARRAY {
		fix_array_size(t.ptr)
		t.size = t.len * t.ptr.size
	} else if t.typ == CTYPE_PTR {
		fix_array_size(t.ptr)
	} else if t.typ == CTYPE_FUNC {
		fix_array_size(t.rettype)
	}
}

func read_declarator(rname *string, basetype *Ctype, params []*Ast, ctx int) (*Ctype, []*Ast) {
	t, params := read_direct_declarator1(rname, basetype, params, ctx)
	fix_array_size(t)
	return t, params
}

var kconst int
var kvolatile int
var kinline int

func read_decl_spec() (*Ctype, int) {
	var sclass int

	tok := peek_token()
	if tok == nil || tok.typ != TTYPE_IDENT {
		return nil, 0
	}

	var tmp *Ctype
	var usertype *Ctype

	type sign int
	const (
		ksigned = sign(iota + 1)
		kunsigned
	)
	var sig sign

	type ttype int
	const (
		kvoid = ttype(iota + 1)
		kchar
		kint
		kfloat
		kdouble
	)
	const (
		kshort = ttype(iota + 1)
		klong
		kllong
	)
	var typ ttype
	var size ttype

	myerror := func (tok *Token) {
		errorf("internal error")
	}
	check := func() {
		if size == kshort && (typ != 0 && typ != kint) {
			myerror(tok)
		}
		if size == klong && (typ != 0 && typ != kint && typ != kdouble) {
			myerror(tok)
		}
		if sig != 0 && (typ == kvoid || typ == kfloat || typ == kdouble) {
			myerror(tok)
		}
		if usertype != nil && (typ != 0 || size != 0 || sig != 0) {
			myerror(tok)
		}
	}
	setType := func (val ttype) {
		typ = val
		check()
	}
	setSig := func(s sign) {
		sig = s
		check()
	}
	setSize := func(s ttype) {
		size = s
		check()
	}
	setUserType := func(t *Ctype) {
		usertype = t
		check()
	}

	for {
		setsclass := func (val int) {
			if sclass != 0 {
				panic("internal error")
			}
			sclass = val
		}

		tok = read_token()
		if tok == nil {
			errorf("premature end of input")
		}
		if tok.typ != TTYPE_IDENT {
			unget_token(tok)
			break
		}
		s := tok.sval
		if s == "typedef" {
			setsclass(S_TYPEDEF)
		} else if s == "extern" {
			setsclass(S_EXTERN)
		} else if s == "static" {
			setsclass(S_STATIC)
		} else if s == "auto" {
			setsclass(S_AUTO)
		} else if s == "register" {
			setsclass(S_REGISTER)
		} else if s == "const" {
			kconst = 1
		} else if s == "volatile" {
			kvolatile = 1
		} else if s == "inline" {
			kinline = 1
		} else if s == "static" {
			// ignore
		} else if s == "void" {
			setType(kvoid)
		} else if s == "char" {
			setType(kchar)
		} else if s == "int" {
			setType(kint)
		} else if s == "float" {
			setType(kfloat)
		} else if s == "double" {
			setType(kdouble)
		} else if s == "signed" {
			setSig(ksigned)
		} else if s == "unsigned" {
			setSig(kunsigned)
		} else if s == "short" {
			setSize(kshort)
		} else if s == "struct" {
			setUserType(read_struct_def())
		} else if s == "union" {
			setUserType(read_union_def())
		} else if s == "enum" {
			setUserType(read_enum_def())
		} else if s == "long" {
			if size == 0 {
				setSize(klong)
			} else if size == klong {
				size = kllong
			} else {
				myerror(tok)
			}
		} else if tmp = typedefs.GetCtype(s); tmp != nil {
			setUserType(tmp)
		} else {
			unget_token(tok)
			break
		}
		setsclass = nil
	}

	if usertype != nil {
		return usertype, sclass
	}
	switch typ {
	case kchar:
		return make_type(CTYPE_CHAR, sig != kunsigned), sclass
	case kfloat:
		return make_type(CTYPE_FLOAT, false), sclass
	case kdouble:
		var ctyp int
		if size == klong {
			ctyp = CTYPE_LDOUBLE
		} else {
			ctyp = CTYPE_DOUBLE
		}
		return make_type(ctyp, false), sclass
	}
	switch size {
	case kshort:
		return make_type(CTYPE_SHORT, sig != kunsigned), sclass
	case klong:
		return make_type(CTYPE_LONG, sig != kunsigned), sclass
	case kllong:
		return make_type(CTYPE_LLONG, sig != kunsigned), sclass
	default:
		return make_type(CTYPE_INT, sig != kunsigned ), sclass
	}
	return nil, 0
}

func read_func_param(rtype **Ctype, name *string, optional bool) {
	basetype, _ := read_decl_spec()
	var ctx int
	if optional {
		ctx = DECL_PARAM_TYPEONLY
	} else {
		ctx = DECL_PARAM
	}
	basetype,_ = read_declarator(name, basetype, nil, ctx)
	*rtype = read_array_dimensions(basetype)
}

func read_decl_array_init_val(ctype *Ctype) *Ast {
	var initlist []*Ast
	initlist = read_decl_array_init_int(initlist, ctype)
	init := ast_init_list(initlist)

	var length int
	if init.typ == AST_STRING {
		length = len(init.val) + 1
	} else {
		length = len(init.initlist)
	}
	if ctype.len == -1 {
		ctype.len = length
		ctype.size = length * ctype.ptr.size
	} else if ctype.len != length {
		errorf("Invalid array initializer: expected %d items but got %d",
			ctype.len, length)
	}
	return init
}

func read_decl_struct_init_val(ctype *Ctype) *Ast {
	expect('{')
	var initlist []*Ast
	for _, val := range ctype.fields.Values() {
		fieldtype := val.ctype
		tok := read_token()
		if tok.is_punct('}') {
			return ast_init_list(initlist)
		}
		if tok.is_punct('{') {
			if fieldtype.typ != CTYPE_ARRAY {
				errorf("array expected, but got %s", fieldtype)
			}
			unget_token(tok)
			initlist = read_decl_array_init_int(initlist, fieldtype)
			continue
		}
		unget_token(tok)
		initlist= read_decl_init_elem(initlist, fieldtype)
	}
	expect('}')
	return ast_init_list(initlist)
}

func read_decl_init_val(ctype *Ctype) *Ast {
	var init *Ast
	if ctype.typ == CTYPE_ARRAY {
		init = read_decl_array_init_val(ctype)
	} else if ctype.typ == CTYPE_STRUCT {
		init = read_decl_struct_init_val(ctype)
	} else {
		init = read_expr()
	}
	return init
}

func read_array_dimensions_int(basetype *Ctype) *Ctype {
	tok := read_token()
	if !tok.is_punct('[') {
		unget_token(tok)
		return nil
	}
	dim := -1
	if !peek_token().is_punct(']') {
		size := read_expr()
		dim = eval_intexpr(size)
	}
	expect(']')
	sub := read_array_dimensions_int(basetype)
	if sub != nil {
		if sub.len == -1 && dim == -1 {
			errorf("Array len is not specified")
		}
		return make_array_type(sub, dim)
	}

	return make_array_type(basetype, dim)
}

func read_array_dimensions(basetype *Ctype) *Ctype {
	ctype := read_array_dimensions_int(basetype)
	if ctype == nil {
		return basetype
	}
	return ctype
}

func read_decl_init(variable *Ast) *Ast {
	init := read_decl_init_val(variable.ctype)
	if variable.typ == AST_GVAR && is_inttype(variable.ctype) {
		init = ast_inttype(ctype_int, eval_intexpr(init))
	}
	return ast_decl(variable, init)
}

func read_if_stmt() *Ast {
	expect('(')
	cond := read_expr()
	expect(')')
	then := read_stmt()
	tok := read_token()
	if tok == nil || !tok.is_ident_type() || tok.sval != "else" {
		unget_token(tok)
		return ast_if(cond, then, nil)
	}
	els := read_stmt()
	return ast_if(cond, then, els)
}

func read_opt_decl_or_stmt() *Ast {
	tok := read_token()
	if tok.is_punct(';') {
		return nil
	}
	unget_token(tok)
	var list []*Ast = make([]*Ast,0)
	read_decl_or_stmt(&list)
	return list[0]

}

func read_opt_expr() *Ast {
	tok := read_token()
	if tok.is_punct(';') {
		return nil
	}
	unget_token(tok)
	r := read_expr()
	expect(';')
	return r
}

func read_for_stmt() *Ast {
	expect('(')
	localenv = MakeDict(localenv)
	init := read_opt_decl_or_stmt()
	cond := read_opt_expr()
	var step *Ast
	if peek_token().is_punct(')') {
		step = nil
	} else {
		step = read_expr()
	}
	expect(')')
	body := read_stmt()
	localenv = localenv.Parent()
	return ast_for(init, cond, step, body)
}

func read_return_stmt() *Ast {
	retval := read_expr()
	expect(';')
	return ast_return(current_func_type.rettype, retval)
}

func read_stmt() *Ast {
	tok := read_token()
	if tok.is_ident("if") {
		return read_if_stmt()
	}
	if tok.is_ident("for") {
		return read_for_stmt()
	}
	if tok.is_ident("return") {
		return read_return_stmt()
	}
	if tok.is_punct('{') {
		return read_compound_stmt()
	}
	unget_token(tok)
	r := read_expr()
	expect(';')
	return r
}

func read_decl_or_stmt(list *[]*Ast) {
	tok := peek_token()
	if tok == nil {
		errorf("premature end of input")
	}
	if is_type_keyword(tok) {
		*list = read_decl(*list, ast_lvar)
	} else {
		*list = append(*list, read_stmt())
	}
}

func read_compound_stmt() *Ast {
	localenv = MakeDict(localenv)
	var list []*Ast

	for {
		read_decl_or_stmt(&list)
		tok := read_token()
		if tok.is_punct('}') {
			break
		}
		unget_token(tok)
	}
	localenv = localenv.Parent()
	return ast_compound_stmt(list)
}

func read_func_param_list(rettype *Ctype, paramvars []*Ast) (*Ctype, []*Ast) {
	typeonly := (paramvars == nil)
	var paramtypes []*Ctype
	var rtype *Ctype
	pt := read_token()
	if pt.is_punct(')') {
		rtype = make_func_type(rettype, paramtypes, false)
		return rtype, nil
	}
	unget_token(pt)
	for {
		pt = read_token()
		if pt.is_ident("...") {
			if len(paramtypes) == 0 {
				errorf("at least one parameter is required")
			}
			expect(')')
			rtype = make_func_type(rettype, paramtypes, true)
			return rtype, paramvars
		} else {
			unget_token(pt)
		}
		var ptype *Ctype
		var name string
		read_func_param(&ptype, &name, typeonly)
		if ptype.typ == CTYPE_ARRAY {
			ptype = make_ptr_type(ptype.ptr)
		}
		paramtypes = append(paramtypes, ptype)
		if !typeonly {
			paramvars = append(paramvars, ast_lvar(ptype, name))
		}
		tok := read_token()
		if tok.is_punct(')') {
			rtype = make_func_type(rettype, paramtypes, false)
			return rtype, paramvars
		}
		if !tok.is_punct(',') {
			errorf("comma expected, but got %s", tok)
		}
	}
}

func read_func_body(functype *Ctype, fname string, params []*Ast) *Ast {
	localenv = MakeDict(localenv)
	localvars = make([]*Ast, 0)
	current_func_type = functype
	body := read_compound_stmt()
	r := ast_func(functype, fname, params, localvars, body)
	globalenv.PutAst(fname, r)
	current_func_type = nil
	localenv = nil
	localvars = nil
	return r
}

func is_funcdef() bool {
	buf := make(TokenList, 0)
	nest := 0
	paren := false
	r := true
	for {
		tok := read_token()
		buf = append(buf, tok)
		if tok == nil {
			errorf("premature end of input")
		}
		if nest == 0 && paren && tok.is_punct('{') {
			break
		}
		if nest == 0 && (tok.is_punct(';') || tok.is_punct(',') || tok.is_punct('=')) {
			r = false
			break
		}
		if tok.is_punct('(') {
			nest++
		}
		if tok.is_punct(')') {
			if nest == 0 {
				errorf("extra close parenthesis")
			}
			paren = true
			nest--
		}
	}
	for i:= len(buf) - 1; i >= 0 ;i-- {
		unget_token(buf[i])
	}

	return r
}

func read_funcdef() *Ast {
	var name string
	basetype, _ := read_decl_spec()
	localenv = MakeDict(globalenv)
	var params []*Ast = make([]*Ast, 0)
	functype, params := read_declarator(&name, basetype, params, DECL_BODY)
	expect('{')
	r := read_func_body(functype, name, params)
	localenv = nil
	return r
}

func read_decl(block []*Ast, make_var MakeVarFn) []*Ast {
	basetype, sclass := read_decl_spec()
	tok := read_token()
	if tok.is_punct(';') {
		return nil
	}
	unget_token(tok)
	for {
		var name string
		ctype, _ := read_declarator(&name, basetype, nil, DECL_BODY)
		tok = read_token()
		if tok.is_punct('=') {
			if sclass == S_TYPEDEF {
				errorf("= after typedef")
			}
			gvar := make_var(ctype, name)
			block = append(block, read_decl_init(gvar))
			tok = read_token()
		} else if sclass == S_TYPEDEF {
			typedefs.PutCtype(name, ctype)
		} else if ctype.typ == CTYPE_FUNC {
			make_var(ctype, name)
		} else {
			gvar := make_var(ctype, name)
			if sclass != S_EXTERN {
				block = append(block, ast_decl(gvar, nil))
			}
		}

		if tok.is_punct(';') {
			return block
		}
		if !tok.is_punct(',') {
			errorf("Don't know how to handle %s", tok)
		}
	}
}

func read_toplevels() []*Ast {
	var r []*Ast
	for {
		if peek_token() == nil {
			return r
		}
		if is_funcdef() {
			r = append(r, read_funcdef())
		} else {
			r = read_decl(r, ast_gvar)
		}
	}
}
