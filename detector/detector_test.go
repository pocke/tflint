package detector

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/parser"
	"github.com/hashicorp/hcl/hcl/token"
	"github.com/wata727/tflint/config"
	"github.com/wata727/tflint/evaluator"
	"github.com/wata727/tflint/issue"
	"github.com/wata727/tflint/logger"
)

func TestDetect(t *testing.T) {
	type Config struct {
		IgnoreRule   string
		IgnoreModule string
	}

	cases := []struct {
		Name   string
		Config Config
		Result int
	}{
		{
			Name: "detect template and module",
			Config: Config{
				IgnoreRule:   "",
				IgnoreModule: "",
			},
			Result: 2,
		},
		{
			Name: "ignore module",
			Config: Config{
				IgnoreRule:   "",
				IgnoreModule: "./tf_aws_ec2_instance",
			},
			Result: 1,
		},
		{
			Name: "ignore rule",
			Config: Config{
				IgnoreRule:   "test_rule",
				IgnoreModule: "",
			},
			Result: 0,
		},
	}

	detectors = map[string]string{
		"test_rule": "DetectMethodForTest",
	}

	for _, tc := range cases {
		prev, _ := filepath.Abs(".")
		dir, _ := os.Getwd()
		defer os.Chdir(prev)
		testDir := dir + "/test-fixtures"
		os.Chdir(testDir)

		listMap := make(map[string]*ast.ObjectList)
		root, _ := parser.Parse([]byte(`
module "ec2_instance" {
    source = "./tf_aws_ec2_instance"
    ami = "ami-12345"
    num = "1"
}`))
		list, _ := root.Node.(*ast.ObjectList)
		listMap["text.tf"] = list

		c := config.Init()
		c.SetIgnoreRule(tc.Config.IgnoreRule)
		c.SetIgnoreModule(tc.Config.IgnoreModule)
		evalConfig, _ := evaluator.NewEvaluator(listMap, c)
		d := &Detector{
			ListMap:    listMap,
			Config:     c,
			EvalConfig: evalConfig,
			Logger:     logger.Init(false),
		}

		issues := d.Detect()
		if len(issues) != tc.Result {
			t.Fatalf("Bad: %s\nExpected: %s\n\ntestcase: %s", len(issues), tc.Result, tc.Name)
		}
	}
}

func (d *Detector) DetectMethodForTest(issues *[]*issue.Issue) {
	*issues = append(*issues, &issue.Issue{
		Type:    "TEST",
		Message: "this is test method",
		Line:    1,
		File:    "",
	})
}

func TestHclLiteralToken(t *testing.T) {
	type Input struct {
		File string
		Key  string
	}

	cases := []struct {
		Name   string
		Input  Input
		Result token.Token
		Error  bool
	}{
		{
			Name: "return literal token",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    instance_type = "t2.micro"
}`,
				Key: "instance_type",
			},
			Result: token.Token{
				Type: 9,
				Pos: token.Pos{
					Filename: "",
					Offset:   47,
					Line:     3,
					Column:   21,
				},
				Text: "\"t2.micro\"",
				JSON: false,
			},
			Error: false,
		},
		{
			Name: "happen error when value is list",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    instance_type = ["t2.micro"]
}`,
				Key: "instance_type",
			},
			Result: token.Token{},
			Error:  true,
		},
		{
			Name: "happen error when value is map",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    instance_type = {
        default = "t2.micro"
    }
}`,
				Key: "instance_type",
			},
			Result: token.Token{},
			Error:  true,
		},
		{
			Name: "happen error when key not found",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    instance_type = "t2.micro"
}`,
				Key: "ami_id",
			},
			Result: token.Token{},
			Error:  true,
		},
	}

	for _, tc := range cases {
		root, _ := parser.Parse([]byte(tc.Input.File))
		list, _ := root.Node.(*ast.ObjectList)
		item := list.Filter("resource", "aws_instance").Items[0]

		result, err := hclLiteralToken(item, tc.Input.Key)
		if tc.Error == true && err == nil {
			t.Fatalf("should be happen error.\n\ntestcase: %s", tc.Name)
			continue
		}
		if tc.Error == false && err != nil {
			t.Fatalf("should not be happen error.\nError: %s\n\ntestcase: %s", err, tc.Name)
			continue
		}

		if result.Text != tc.Result.Text {
			t.Fatalf("Bad: %s\nExpected: %s\n\ntestcase: %s", result, tc.Result, tc.Name)
		}
	}
}

func TestHclObjectItems(t *testing.T) {
	type Input struct {
		File string
		Key  string
	}

	cases := []struct {
		Name   string
		Input  Input
		Result []*ast.ObjectItem
		Error  bool
	}{
		{
			Name: "return object items",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    root_block_device = {
        volume_size = "16"
    }
}`,
				Key: "root_block_device",
			},
			Result: []*ast.ObjectItem{
				&ast.ObjectItem{
					Keys: []*ast.ObjectKey{},
					Assign: token.Pos{
						Filename: "",
						Offset:   55,
						Line:     3,
						Column:   23,
					},
					Val: &ast.ObjectType{
						Lbrace: token.Pos{
							Filename: "",
							Offset:   57,
							Line:     3,
							Column:   25,
						},
						Rbrace: token.Pos{
							Filename: "",
							Offset:   90,
							Line:     5,
							Column:   5,
						},
						List: &ast.ObjectList{
							Items: []*ast.ObjectItem{
								&ast.ObjectItem{
									Keys: []*ast.ObjectKey{
										&ast.ObjectKey{
											Token: token.Token{
												Type: 4,
												Pos: token.Pos{
													Filename: "",
													Offset:   67,
													Line:     4,
													Column:   9,
												},
												Text: "volume_size",
												JSON: false,
											},
										},
									},
									Assign: token.Pos{
										Filename: "",
										Offset:   79,
										Line:     4,
										Column:   21,
									},
									Val: &ast.LiteralType{
										Token: token.Token{
											Type: 9,
											Pos: token.Pos{
												Filename: "",
												Offset:   81,
												Line:     4,
												Column:   23,
											},
											Text: "\"16\"",
											JSON: false,
										},
										LineComment: (*ast.CommentGroup)(nil),
									},
									LeadComment: (*ast.CommentGroup)(nil),
									LineComment: (*ast.CommentGroup)(nil),
								},
							},
						},
					},
					LeadComment: (*ast.CommentGroup)(nil),
					LineComment: (*ast.CommentGroup)(nil),
				},
			},
			Error: false,
		},
		{
			Name: "happen error when key not found",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    root_block_device = {
        volume_size = "16"
    }
}`,
				Key: "ami_id",
			},
			Result: []*ast.ObjectItem{},
			Error:  true,
		},
	}

	for _, tc := range cases {
		root, _ := parser.Parse([]byte(tc.Input.File))
		list, _ := root.Node.(*ast.ObjectList)
		item := list.Filter("resource", "aws_instance").Items[0]

		result, err := hclObjectItems(item, tc.Input.Key)
		if tc.Error == true && err == nil {
			t.Fatalf("should be happen error.\n\ntestcase: %s", tc.Name)
			continue
		}
		if tc.Error == false && err != nil {
			t.Fatalf("should not be happen error.\nError: %s\n\ntestcase: %s", err, tc.Name)
			continue
		}

		if !reflect.DeepEqual(result, tc.Result) {
			t.Fatalf("Bad: %s\nExpected: %s\n\ntestcase: %s", result, tc.Result, tc.Name)
		}
	}
}

func TestIsKeyNotFound(t *testing.T) {
	type Input struct {
		File string
		Key  string
	}

	cases := []struct {
		Name   string
		Input  Input
		Result bool
	}{
		{
			Name: "key found",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    instance_type = "t2.micro"
}`,
				Key: "instance_type",
			},
			Result: false,
		},
		{
			Name: "happen error when value is list",
			Input: Input{
				File: `
resource "aws_instance" "web" {
    instance_type = "t2.micro"
}`,
				Key: "iam_instance_profile",
			},
			Result: true,
		},
	}

	for _, tc := range cases {
		root, _ := parser.Parse([]byte(tc.Input.File))
		list, _ := root.Node.(*ast.ObjectList)
		item := list.Filter("resource", "aws_instance").Items[0]
		result := IsKeyNotFound(item, tc.Input.Key)

		if result != tc.Result {
			t.Fatalf("Bad: %s\nExpected: %s\n\ntestcase: %s", result, tc.Result, tc.Name)
		}
	}
}

func TestEvalToString(t *testing.T) {
	type Input struct {
		Src  string
		File string
	}

	cases := []struct {
		Name   string
		Input  Input
		Result string
		Error  bool
	}{
		{
			Name: "return string",
			Input: Input{
				Src: "${var.text}",
				File: `
variable "text" {
    default = "result"
}`,
			},
			Result: "result",
			Error:  false,
		},
		{
			Name: "not string",
			Input: Input{
				Src: "${var.text}",
				File: `
variable "text" {
    default = ["result"]
}`,
			},
			Result: "",
			Error:  true,
		},
		{
			Name: "not evaluable",
			Input: Input{
				Src:  "${aws_instance.app}",
				File: `variable "text" {}`,
			},
			Result: "",
			Error:  true,
		},
	}

	for _, tc := range cases {
		listMap := make(map[string]*ast.ObjectList)
		root, _ := parser.Parse([]byte(tc.Input.File))
		list, _ := root.Node.(*ast.ObjectList)
		listMap["text.tf"] = list

		evalConfig, _ := evaluator.NewEvaluator(listMap, config.Init())
		d := &Detector{
			ListMap:    listMap,
			EvalConfig: evalConfig,
		}

		result, err := d.evalToString(tc.Input.Src)
		if tc.Error == true && err == nil {
			t.Fatalf("should be happen error.\n\ntestcase: %s", tc.Name)
			continue
		}
		if tc.Error == false && err != nil {
			t.Fatalf("should not be happen error.\nError: %s\n\ntestcase: %s", err, tc.Name)
			continue
		}

		if result != tc.Result {
			t.Fatalf("Bad: %s\nExpected: %s\n\ntestcase: %s", result, tc.Result, tc.Name)
		}
	}
}
