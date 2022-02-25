// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genhcl_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/test"
	"github.com/mineiros-io/terramate/test/hclwrite"
	"github.com/mineiros-io/terramate/test/sandbox"
	"github.com/rs/zerolog"
)

func TestLoadGeneratedHCL(t *testing.T) {
	type (
		hclconfig struct {
			path     string
			filename string
			add      fmt.Stringer
		}
		genHCL struct {
			body   fmt.Stringer
			origin string
		}
		result struct {
			name string
			hcl  genHCL
		}
		testcase struct {
			name    string
			stack   string
			configs []hclconfig
			want    []result
			wantErr error
		}
	)

	hcldoc := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildHCL(builders...)
	}
	generateHCL := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("generate_hcl", builders...)
	}
	block := func(name string, builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock(name, builders...)
	}
	terraform := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("terraform", builders...)
	}
	globals := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("globals", builders...)
	}
	content := func(builders ...hclwrite.BlockBuilder) *hclwrite.Block {
		return hclwrite.BuildBlock("content", builders...)
	}
	attr := func(name, expr string) hclwrite.BlockBuilder {
		t.Helper()
		return hclwrite.AttributeValue(t, name, expr)
	}
	labels := func(labels ...string) hclwrite.BlockBuilder {
		return hclwrite.Labels(labels...)
	}
	expr := hclwrite.Expression
	str := hclwrite.String
	number := hclwrite.NumberInt
	boolean := hclwrite.Boolean

	defaultCfg := func(dir string) string {
		return filepath.Join(dir, config.DefaultFilename)
	}

	tcases := []testcase{
		{
			name:  "no generation",
			stack: "/stack",
		},
		{
			name:  "empty content block generates empty code",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("empty"),
						content(),
					),
				},
			},
			want: []result{
				{
					name: "empty",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body:   hcldoc(),
					},
				},
			},
		},
		{
			name:  "generate hcl on stack with single empty block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("emptytest"),
						content(
							block("empty"),
						),
					),
				},
			},
			want: []result{
				{
					name: "emptytest",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body:   block("empty"),
					},
				},
			},
		},
		{
			name:  "generate hcl with only attributes on root body",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("attrs"),
						content(
							number("num", 666),
							str("str", "hi"),
						),
					),
				},
			},
			want: []result{
				{
					name: "attrs",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: hcldoc(
							number("num", 666),
							str("str", "hi"),
						),
					},
				},
			},
		},
		{
			name:  "generate hcl with attributes and blocks on root body",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("attrs"),
						content(
							number("num", 666),
							block("test"),
							str("str", "hi"),
						),
					),
				},
			},
			want: []result{
				{
					name: "attrs",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: hcldoc(
							number("num", 666),
							str("str", "hi"),
							block("test"),
						),
					},
				},
			},
		},
		{
			name:  "scope traversals of unknown namespaces are copied as is",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("scope_traversal"),
						content(
							block("traversals",
								expr("local", "local.something"),
								expr("mul", "omg.wat.something"),
								expr("res", "resource.something"),
								expr("val", "omg.something"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "scope_traversal",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: block("traversals",
							expr("local", "local.something"),
							expr("mul", "omg.wat.something"),
							expr("res", "resource.something"),
							expr("val", "omg.something"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with single block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stack",
					add: generateHCL(
						labels("test"),
						content(
							block("testblock",
								expr("bool", "global.some_bool"),
								expr("number", "global.some_number"),
								expr("string", "global.some_string"),
								expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: block("testblock",
							boolean("bool", true),
							number("number", 777),
							attr("obj", `{
								bool   = true
								number = 777
								string = "string"
							}`),
							str("string", "string"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on root with multiple files",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/",
					filename: "root.tm.hcl",
					add: generateHCL(
						labels("test"),
						content(
							block("testblock",
								expr("bool", "global.some_bool"),
								expr("number", "global.some_number"),
								expr("string", "global.some_string"),
							),
						),
					),
				},
				{
					path:     "/",
					filename: "root2.tm.hcl",
					add: generateHCL(
						labels("test2"),
						content(
							block("testblock2",
								expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin: "/root.tm.hcl",
						body: block("testblock",
							boolean("bool", true),
							number("number", 777),
							str("string", "string"),
						),
					},
				},
				{
					name: "test2",
					hcl: genHCL{
						origin: "/root2.tm.hcl",
						body: block("testblock2",
							attr("obj", `{
								bool   = true
								number = 777
								string = "string"
							}`),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with multiple files",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "test.tm.hcl",
					add: generateHCL(
						labels("test"),
						content(
							block("testblock",
								expr("bool", "global.some_bool"),
								expr("number", "global.some_number"),
								expr("string", "global.some_string"),
							),
						),
					),
				},
				{
					path:     "/stack",
					filename: "test2.tm.hcl",
					add: generateHCL(
						labels("test2"),
						content(
							block("testblock2",
								expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin: "/stack/test.tm.hcl",
						body: block("testblock",
							boolean("bool", true),
							number("number", 777),
							str("string", "string"),
						),
					},
				},
				{
					name: "test2",
					hcl: genHCL{
						origin: "/stack/test2.tm.hcl",
						body: block("testblock2",
							attr("obj", `{
								bool   = true
								number = 777
								string = "string"
							}`),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack using try and labeled block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						attr("obj", `{
							field_a = "a"
							field_b = "b"
							field_c = "c"
						}`),
					),
				},
				{
					path: "/stack",
					add: generateHCL(
						labels("test"),
						content(
							block("labeled",
								labels("label1", "label2"),
								expr("field_a", "try(global.obj.field_a, null)"),
								expr("field_b", "try(global.obj.field_b, null)"),
								expr("field_c", "try(global.obj.field_c, null)"),
								expr("field_d", "try(global.obj.field_d, null)"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "test",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: block("labeled",
							labels("label1", "label2"),
							str("field_a", "a"),
							str("field_b", "b"),
							str("field_c", "c"),
							attr("field_d", "null"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack with single nested block",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path:     "/stack",
					filename: "genhcl.tm.hcl",
					add: generateHCL(
						labels("nesting"),
						content(
							block("block1",
								expr("bool", "global.some_bool"),
								block("block2",
									expr("number", "global.some_number"),
									block("block3",
										expr("string", "global.some_string"),
										expr("obj", `{
											string = global.some_string
											number = global.some_number
											bool = global.some_bool
										}`),
									),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "nesting",
					hcl: genHCL{
						origin: "/stack/genhcl.tm.hcl",
						body: block("block1",
							boolean("bool", true),
							block("block2",
								number("number", 777),
								block("block3",
									attr("obj", `{
										bool   = true
										number = 777
										string = "string"
									}`),
									str("string", "string"),
								),
							),
						),
					},
				},
			},
		},
		{
			name:  "multiple generate HCL blocks on single file",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: hcldoc(
						globals(
							str("some_string", "string"),
							number("some_number", 666),
							boolean("some_bool", true),
						),
						generateHCL(
							labels("exported_one"),
							content(
								block("block1",
									expr("bool", "global.some_bool"),
									block("block2",
										expr("number", "global.some_number"),
									),
								),
							),
						),
						generateHCL(
							labels("exported_two"),
							content(
								block("yay",
									expr("data", "global.some_string"),
								),
							),
						),
						generateHCL(
							labels("exported_three"),
							content(
								block("something",
									expr("number", "global.some_number"),
								),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "exported_one",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: block("block1",
							boolean("bool", true),
							block("block2",
								number("number", 666),
							),
						),
					},
				},
				{
					name: "exported_two",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: block("yay",
							str("data", "string"),
						),
					},
				},
				{
					name: "exported_three",
					hcl: genHCL{
						origin: defaultCfg("/stack"),
						body: block("something",
							number("number", 666),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on stack parent dir",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: globals(
						str("some_string", "string"),
						number("some_number", 777),
						boolean("some_bool", true),
					),
				},
				{
					path: "/stacks",
					add: generateHCL(
						labels("on_parent"),
						content(
							block("on_parent_block",
								expr("obj", `{
									string = global.some_string
									number = global.some_number
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "on_parent",
					hcl: genHCL{
						origin: defaultCfg("/stacks"),
						body: block("on_parent_block",
							attr("obj", `{
								bool   = true
								number = 777
								string = "string"
							}`),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on project root dir",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: generateHCL(
						labels("root"),
						content(
							block("root",
								expr("test", "terramate.path"),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "root",
					hcl: genHCL{
						origin: defaultCfg("/"),
						body: block("root",
							str("test", "/stacks/stack"),
						),
					},
				},
			},
		},
		{
			name:  "generate HCL on all dirs of the project with different labels",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: hcldoc(
						globals(
							str("some_string", "string"),
							number("some_number", 777),
							boolean("some_bool", true),
						),
						generateHCL(
							labels("on_root"),
							content(
								block("on_root_block",
									expr("obj", `{
										string = global.some_string
									}`),
								),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: generateHCL(
						labels("on_parent"),
						content(
							block("on_parent_block",
								expr("obj", `{
									number = global.some_number
								}`),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("on_stack"),
						content(
							block("on_stack_block",
								expr("obj", `{
									bool = global.some_bool
								}`),
							),
						),
					),
				},
			},
			want: []result{
				{
					name: "on_root",
					hcl: genHCL{
						origin: defaultCfg("/"),
						body: block("on_root_block",
							attr("obj", `{
								string = "string"
							}`),
						),
					},
				},
				{
					name: "on_parent",
					hcl: genHCL{
						origin: defaultCfg("/stacks"),
						body: block("on_parent_block",
							attr("obj", `{
								number = 777
							}`),
						),
					},
				},
				{
					name: "on_stack",
					hcl: genHCL{
						origin: defaultCfg("/stacks/stack"),
						body: block("on_stack_block",
							attr("obj", `{
								bool   = true
							}`),
						),
					},
				},
			},
		},
		{
			name:  "stack with block with same label as parent is an error",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "parent data"),
							),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "stack data"),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrMultiLevelConflict,
		},
		{
			name:  "stack parents with block with same label is an error",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/",
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "root data"),
							),
						),
					),
				},
				{
					path: "/stacks",
					add: generateHCL(
						labels("repeated"),
						content(
							block("block",
								str("data", "parent data"),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrMultiLevelConflict,
		},
		{
			name:  "block with no label fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: generateHCL(
						content(
							block("block",
								str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "generate_hcl with non-content block inside fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("test"),
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "generate_hcl with other blocks than content fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: generateHCL(
						labels("test"),
						content(
							str("data", "some literal data"),
						),
						block("block",
							str("data", "some literal data"),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "generate_hcl.content block is required",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("empty"),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "generate_hcl.content block with label fails",
			stack: "/stack",
			configs: []hclconfig{
				{
					path: "/stack",
					add: generateHCL(
						labels("empty"),
						content(
							labels("not allowed"),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "block with two labels on stack fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("generate_hcl",
						labels("one", "two"),
						content(
							block("block",
								str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "block with empty label on stack fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: block("generate_hcl",
						labels(""),
						content(
							block("block",
								str("data", "some literal data"),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "blocks with same label on same config fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data2", "some literal data2"),
								),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "blocks with same label on multiple config files fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path:     "/stacks/stack",
					filename: "test.tm.hcl",
					add: hcldoc(
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
					),
				},
				{
					path:     "/stacks/stack",
					filename: "test2.tm.hcl",
					add: hcldoc(
						generateHCL(
							labels("duplicated"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "global evaluation failure",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("test"),
							content(
								terraform(
									expr("required_version", "global.undefined"),
								),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrEval,
		},
		{
			name:  "metadata evaluation failure",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("test"),
							content(
								terraform(
									expr("much_wrong", "terramate.undefined"),
								),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrEval,
		},
		{
			name:  "valid config on stack but invalid on parent fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks",
					add: generateHCL(
						block("block",
							str("data", "some literal data"),
						),
					),
				},
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("valid"),
							content(
								terraform(
									str("data", "some literal data"),
								),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
		{
			name:  "attributes on generate_hcl block fails",
			stack: "/stacks/stack",
			configs: []hclconfig{
				{
					path: "/stacks/stack",
					add: hcldoc(
						generateHCL(
							labels("test"),
							str("some_attribute", "whatever"),
							content(
								terraform(
									str("required_version", "1.11"),
								),
							),
						),
					),
				},
			},
			wantErr: genhcl.ErrParsing,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			s := sandbox.New(t)
			stackEntry := s.CreateStack(tcase.stack)
			stack := stackEntry.Load()

			for _, cfg := range tcase.configs {
				filename := cfg.filename
				if filename == "" {
					filename = config.DefaultFilename
				}
				path := filepath.Join(s.RootDir(), cfg.path)
				test.AppendFile(t, path, filename, cfg.add.String())
			}

			meta := stack.Meta()
			globals := s.LoadStackGlobals(meta)
			res, err := genhcl.Load(s.RootDir(), meta, globals)
			assert.IsError(t, err, tcase.wantErr)

			got := res.GeneratedHCLs()

			for _, res := range tcase.want {
				gothcl, ok := got[res.name]
				if !ok {
					t.Fatalf("want hcl code to be generated for %q but no code was generated for it", res.name)
				}
				gotcode := gothcl.String()
				wantcode := res.hcl.body.String()

				assertHCLEquals(t, gotcode, wantcode)
				assert.EqualStrings(t,
					res.hcl.origin,
					gothcl.Origin(),
					"wrong origin config path for generated code",
				)

				delete(got, res.name)
			}

			assert.EqualInts(t, 0, len(got), "got unexpected exported code: %v", got)
		})
	}
}

func assertHCLEquals(t *testing.T, got string, want string) {
	t.Helper()

	const trimmedChars = "\n "

	got = strings.Trim(got, trimmedChars)
	want = strings.Trim(want, trimmedChars)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("generated code doesn't match expectation")
		t.Errorf("want:\n%q", want)
		t.Errorf("got:\n%q", got)
		t.Fatalf("diff:\n%s", diff)
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}