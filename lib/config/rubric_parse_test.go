package config

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"
)

var TestRubric = "testdata/sample_rubric.toml"

func init() {
	_, parentDir, _, _ := runtime.Caller(0)
	parentDir = filepath.Dir(parentDir)
	TestRubric = filepath.Join(parentDir, TestRubric)
}

func TestDecodeRubric(t *testing.T) {
	var rubric Rubric
	expected := Rubric{
		Assignment: "sample assignment!",
		Grader:     "ezr",
		Grade: []Grade{
			Grade{
				Problem: "is-cookie-good?",
				Comment: `Comments for problem 1 here!

wow you did so well!
`,
				Score:    GradeNum(40),
				Possible: GradeNum(70),
			},
			Grade{
				Problem: "collab policy stuff",
				Comment: `Make sure to read the collaboration policy!
`,
				Score:    GradeNum(0.5),
				Possible: GradeNum(15),
			},
		},
	}
	var testStr []byte
	var err error

	if testStr, err = ioutil.ReadFile(TestRubric); err != nil {
		t.Fatalf("Cannot find %v file!", TestRubric)
	}

	if _, err = toml.Decode(string(testStr), &rubric); err != nil {
		t.Fatalf("Error decoding file:\n\t %v", err)
	}

	if !reflect.DeepEqual(rubric, expected) {
		t.Fatalf("Expected\n%v\n, Got \n%v\n", expected, rubric)
	}
}

func TestConformanceRubric(t *testing.T) {
	var rubric Rubric
	var rubric2 Rubric
	var testStr []byte
	var err error
	var b bytes.Buffer

	if testStr, err = ioutil.ReadFile(TestRubric); err != nil {
		t.Fatalf("Cannot find %v file!", TestRubric)
	}

	//fmt.Println(string(testStr))
	if _, err = toml.Decode(string(testStr), &rubric); err != nil {
		t.Fatalf("Error decoding file:\n\t %v", err)
	}

	rubric.WriteTOML(&b)

	if _, err = toml.DecodeReader(&b, &rubric2); err != nil {
		t.Fatalf("Error decoding re-serialized TOML:\n\t %v", err)
	}

	if !reflect.DeepEqual(rubric, rubric2) {
		t.Fatalf("Expected\n%v\n, Got \n%v\n", rubric, rubric2)
	}

}
