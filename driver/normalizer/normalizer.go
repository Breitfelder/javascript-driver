package normalizer

import (
	"strings"

	"gopkg.in/bblfsh/sdk.v2/uast"
	"gopkg.in/bblfsh/sdk.v2/uast/nodes"
	. "gopkg.in/bblfsh/sdk.v2/uast/transformer"
)

var Preprocess = Transformers([][]Transformer{
	{Mappings(Preprocessors...)},
}...)

var Normalize = Transformers([][]Transformer{
	{Mappings(Normalizers...)},
}...)

// Preprocessors is a block of AST preprocessing rules rules.
var Preprocessors = []Mapping{
	// ObjectToNode defines how to normalize common fields of native AST
	// (like node type, token, positional information).
	//
	// https://godoc.org/gopkg.in/bblfsh/sdk.v2/uast#ObjectToNode
	ObjectToNode{
		InternalTypeKey: "type",
		OffsetKey:       "start",
		EndOffsetKey:    "end",
	}.Mapping(),

	Map(
		Part("_", Obj{"loc": AnyNode(nil)}),
		Part("_", Obj{}),
	),
	// preserve raw string and regexp literals
	Map(
		Part("_", Obj{
			uast.KeyType: String("StringLiteral"),
			"value": Any(),
			"extra": Fields{
				{Name: "raw", Op: Var("raw")},
				{Name: "rawValue", Op: Any()},
				//TODO(bzz): make sure parenthesis mapping is consistent \w other drivers
				{Name: "parenthesized", Drop: true, Op: Any()},
				{Name: "parenStart", Drop: true, Op: Any()},
			},
		}),
		Part("_", Obj{
			uast.KeyType: String("StringLiteral"),
			"value":      Var("raw"),
		}),
	),
	Map(
		Part("_", Obj{
			uast.KeyType: String("RegExpLiteral"),
			"extra": Fields{
				{Name: "raw", Op: Var("raw")},
				//TODO(bzz): make sure parenthesis mapping is consistent \w other drivers
				{Name: "parenthesized", Drop: true, Op: Any()},
				{Name: "parenStart", Drop: true, Op: Any()},
			},
		}),
		Part("_", Obj{
			uast.KeyType: String("RegExpLiteral"),
			"raw":        Var("raw"),
		}),
	),
	// drop extra info for other nodes (it duplicates other node fields)
	Map(
		Part("_", Obj{"extra": Any()}),
		Part("_", Obj{}),
	),
}

// Normalizers is the main block of normalization rules to convert native AST to semantic UAST.
var Normalizers = []Mapping{
	MapSemantic("Identifier", uast.Identifier{}, MapObj(
		Fields{
			{Name: "name", Op: Var("name")},
			//FIXME(bzz): map Flow variable types properly
			{Name: "typeAnnotation", Drop: true, Op: Any()},
			//FIXME(bzz): map Flow "Optional Prameter" properly
			{Name: "optional", Drop: true, Op: Any()},
			//FIXME(bzz): map both once we agree how
			{Name: "leadingComments", Drop: true, Op: Any()},
			{Name: "trailingComments", Drop: true, Op: Any()},
		},
		Obj{
			"Name": Var("name"),
		},
	)),
	MapSemantic("JSXIdentifier", uast.Identifier{}, MapObj(
		Obj{
			"name": Var("name"),
		},
		Obj{
			"Name": Var("name"),
		},
	)),
	MapSemantic("StringLiteral", uast.String{}, MapObj(
		Fields{
			{Name: "value", Op: singleQuote{Var("val")}},
			//FIXME(bzz): save both once we agree how
			{Name: "leadingComments", Drop: true, Op: Any()},
			{Name: "trailingComments", Drop: true, Op: Any()},
		},
		Obj{
			"Value":  Var("val"),
			"Format": String("single"),
		},
	)),
	MapSemantic("StringLiteral", uast.String{}, MapObj(
		Fields{
			{Name: "value", Op: Quote(Var("val"))},
			//FIXME(bzz): save both once we agree how
			{Name: "leadingComments", Drop: true, Op: Any()},
			{Name: "trailingComments", Drop: true, Op: Any()},
		},
		Obj{
			"Value": Var("val"),
		},
	)),
	MapSemantic("CommentLine", uast.Comment{}, MapObj(
		Obj{
			"value": CommentText([2]string{"", ""}, "comm"),
		},
		CommentNode(false, "comm", nil),
	)),
	MapSemantic("CommentBlock", uast.Comment{}, MapObj(
		Obj{
			"value": CommentText([2]string{"", ""}, "comm"),
		},
		CommentNode(true, "comm", nil),
	)),
	MapSemantic("BlockStatement", uast.Block{}, MapObj(
		Fields{
			{Name: "body", Op: Var("stmts")},
			{Name: "directives", Op: Arr()}, // TODO: find an example
			//FIXME(bzz): save this once we agree how
			{Name: "trailingComments", Drop: true, Op: Any()},
		},
		Obj{
			"Statements": Var("stmts"),
		},
	)),
	MapSemantic("ImportDeclaration", uast.Import{}, MapObj(
		Fields{
			{Name: "source", Op: Var("path")},
			// empty un-used array
			{Name: "specifiers", Drop: true, Op: Arr()},
			// FIXME(bzz): make sure such comments are linked properly
			{Name: "leadingComments", Drop: true, Op: Any()},
			{Name: "trailingComments", Drop: true, Op: Any()},
		},
		Obj{
			"Path": Var("path"),
		},
	)),
	// importKind switch, set only by flow plugin
	// https://github.com/babel/babel/blob/master/packages/babel-parser/ast/spec.md#importdeclaration
	// TODO(bzz): this mapping misses 'typeof' case
	MapSemantic("ImportDeclaration", uast.Import{}, MapObj(
		CasesObj("case",
			// common
			Fields{
				{Name: "source", Op: Var("path")},
				// FIXME(bzz): make sure such comments are linked properly
				{Name: "leadingComments", Drop: true, Op: Any()},
				{Name: "trailingComments", Drop: true, Op: Any()},
			},
			Objs{
				// namespace
				{
					"importKind": String("value"),
					"specifiers": ArrWith(Var("names"), Fields{
						//uast.KeyType: Var("spec_type"),
						{Name: uast.KeyType, Op: String("ImportNamespaceSpecifier")},
						{Name: uast.KeyPos, Op: Var("local_pos")},
						{Name: "local", Op: Var("local")},
						// FIXME(bzz): make sure such comments are linked properly
						{Name: "leadingComments", Drop: true, Op: Any()},
						{Name: "trailingComments", Drop: true, Op: Any()},
					}),
				},
				// specific type
				{
					"importKind": String("type"),
					"specifiers": ArrWith(Var("names"),
						UASTType(uast.Alias{}, Obj{
							uast.KeyPos: Var("local_pos"),
							"Name":      Var("local"),
							"Node": UASTType(uast.Identifier{}, Obj{
								uast.KeyPos: Any(),
								"Name": String("."),
							}),
						})),
				},
				// normal import
				{
					"importKind": String("value"),
					"specifiers": Check(Not(Arr()), Var("names")),
				},
			},
		),
		CasesObj("case", nil,
			Objs{
				// namespace
				{
					"Path": UASTType(uast.Alias{}, Obj{
						uast.KeyPos: Var("local_pos"),
						"Name":      Var("local"),
						"Node":      Var("path"),
					}),
					"Names": Var("names"),
					"All":   Bool(true),
				},
				// specific type
				{
					"Path": UASTType(uast.Alias{}, Obj{
						uast.KeyPos: Var("local_pos"),
						"Name":      Var("local"),
						"Node":      Var("path"),
					}),
					"Names": Var("names"),
					"All":   Bool(true),
				},
				// normal import
				{
					"Path":  Var("path"),
					"Names": Var("names"),
					"All":   Bool(false),
				},
			},
		),
	)),
	MapSemantic("ImportSpecifier", uast.Alias{}, MapObj(
		Obj{
			"importKind": Is(nil),
			"local":      Var("local"),
			"imported":   Var("imp"),
		},
		Obj{
			"Name": Var("local"),
			"Node": Var("imp"),
		},
	)),
	MapSemantic("ImportDefaultSpecifier", uast.Alias{}, MapObj(
		Fields{
			{Name: "local", Op: Var("local")},
			//FIXME(bzz): save this once we agree how
			{Name: "leadingComments", Drop: true, Op: Any()},
			{Name: "trailingComments", Drop: true, Op: Any()},
		},
		Obj{
			"Name": Var("local"),
			"Node": UASTType(uast.Identifier{}, Obj{
				uast.KeyPos: Any(),
				"Name":      String("."), // TODO: scope
			}),
		},
	)),
	MapSemantic("FunctionDeclaration", uast.FunctionGroup{}, MapObj(
		Fields{
			{Name: "id", Op: Var("name")},
			{Name: "generator", Op: Var("gen")}, // FIXME: define channels in SDK? or return a function?
			{Name: "async", Op: Var("async")},   // TODO: async
			{Name: "body", Op: Var("body")},
			//FIXME(bzz): map Flow predicate properly
			// https://flow.org/en/docs/types/functions/#toc-predicate-functions
			{Name: "predicate", Drop: true, Op: Any()},
			//FIXME(bzz): map Flow return type annotations
			// https://flow.org/en/docs/types/functions/#toc-function-returns
			{Name: "returnType", Drop: true, Op: Any()},
			//FIXME(bzz): map Flow generic types annotations
			// https://flow.org/en/docs/types/generics/
			// see fixtures/ext_typedecl.js#34 func makeWeakCache
			{Name: "typeParameters", Drop: true, Op: Any()},
			// FIXME(bzz): make sure such comments are linked properly
			{Name: "leadingComments", Drop: true, Op: Any()},
			{Name: "trailingComments", Drop: true, Op: Any()},
			{Name: "params", Op: Each("params", Cases("param_case",
				// Identifier
				Check(
					HasType(uast.Identifier{}),
					Var("arg_name"),
				),
				// AssignmentPattern
				Obj{
					uast.KeyType: String("AssignmentPattern"),
					uast.KeyPos:  Var("arg_pos"),
					"left":       Var("arg_name"),
					"right":      Var("arg_init"),
				},
				// RestElement
				Obj{
					uast.KeyType: String("RestElement"),
					uast.KeyPos:  Var("arg_pos"),
					"argument":   Var("arg_name"),
				},
			))},
		},
		Obj{
			"Nodes": Arr(
				Obj{
					"async":     Var("async"),
					"generator": Var("gen"),
				},
				UASTType(uast.Alias{}, Obj{
					"Name": Var("name"),
					"Node": UASTType(uast.Function{}, Obj{
						"Type": UASTType(uast.FunctionType{}, Obj{
							"Arguments": Each("params", Cases("param_case",
								// Identifier
								UASTType(uast.Argument{}, Obj{
									"Name": Var("arg_name"),
								}),
								// AssignmentPattern
								UASTType(uast.Argument{}, Obj{
									uast.KeyPos: Var("arg_pos"),
									"Name":      Var("arg_name"),
									"Init":      Var("arg_init"),
								}),
								// RestElement
								UASTType(uast.Argument{}, Obj{
									uast.KeyPos: Var("arg_pos"),
									"Name":      Var("arg_name"),
									"Variadic":  Bool(true),
								}),
							)),
							"Returns": Arr(
								UASTType(uast.Argument{}, Obj{
									"Init": Is(uast.Identifier{
										Name: "undefined",
									}),
								}),
							),
						}),
						"Body": Var("body"),
					}),
				}),
			),
		},
	)),
}

type singleQuote struct {
	op Op
}

func (op singleQuote) Kinds() nodes.Kind {
	return nodes.KindString
}

func (op singleQuote) Check(st *State, n nodes.Node) (bool, error) {
	sn, ok := n.(nodes.String)
	if !ok {
		return false, nil
	}
	s := string(sn)
	if !strings.HasPrefix(s, `'`) || !strings.HasSuffix(s, `'`) {
		return false, nil
	}
	s, err := unquoteSingle(s)
	if err != nil {
		return false, err
	}
	return op.op.Check(st, nodes.String(s))
}

func (op singleQuote) Construct(st *State, n nodes.Node) (nodes.Node, error) {
	n, err := op.op.Construct(st, n)
	if err != nil {
		return nil, err
	}
	sn, ok := n.(nodes.String)
	if !ok {
		return nil, ErrUnexpectedType.New(nodes.String(""), n)
	}
	s := quoteSingle(string(sn))
	return nodes.String(s), nil
}
