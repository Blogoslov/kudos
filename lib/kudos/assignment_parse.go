package kudos

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/joshlf/kudos/lib/config"
)

// NOTE: All of the convenience methods to retrieve
// fields of the various parseable* types will either:
//   - check to see if the field is set before dereferencing
//     the pointer if the field is optional
//   - assume that the field has been set and dereference
//     the pointer if the field is mandatory
//
// These methods shouldn't be called except for during
// validation (in a manner that makes sure this is safe)
// or after validation (at which point these invariants
// are guaranteed to hold)

type parseableHandin struct {
	Code     *string  `json:"code"`
	Due      *date    `json:"due"`
	Problems []string `json:"problems"`
}

// Convert p to an exported Handin type.
// This function performs no validation,
// so you must do validation independent
// of this function.
func (p parseableHandin) toHandin() (hh Handin) {
	hh.Code = p.code()
	hh.Due = p.due()
	hh.Problems = p.problems()
	return
}

func (p parseableHandin) code() (s string) {
	if p.Code != nil {
		s = *p.Code
	}
	return
}

func (p parseableHandin) due() (t time.Time) {
	if p.Due != nil {
		t = time.Time(*p.Due)
	}
	return
}

func (p parseableHandin) problems() (probs []string) {
	for _, pp := range p.Problems {
		probs = append(probs, pp)
	}
	return
}

func (p parseableHandin) hasCode() bool { return p.Code != nil }
func (p parseableHandin) hasDue() bool  { return p.Due != nil }

type parseableProblem struct {
	Code                  *string            `json:"code"`
	Name                  *string            `json:"name"`
	RubricCommentTemplate *string            `json:"rubric_comment_template"`
	Points                *float64           `json:"points"`
	Subproblems           []parseableProblem `json:"subproblems"`
}

// Convert p to an exported Problem type.
// This function performs no validation,
// so you must do validation independent
// of this function.
func (p parseableProblem) toProblem() (pp Problem) {
	pp.Code = p.code()
	pp.Name = p.name()
	pp.RubricCommentTemplate = p.rubricCommentTemplate()
	pp.Points = p.points()
	for _, ppp := range p.Subproblems {
		pp.Subproblems = append(pp.Subproblems, ppp.toProblem())
	}
	return
}

func (p parseableProblem) code() string { return *p.Code }

func (p parseableProblem) name() (s string) {
	if p.Name != nil {
		s = *p.Name
	}
	return
}

func (p parseableProblem) rubricCommentTemplate() (s string) {
	if p.RubricCommentTemplate != nil {
		s = *p.RubricCommentTemplate
	}
	return
}

func (p parseableProblem) points() float64 { return *p.Points }

func (p parseableProblem) subproblems() (probs []parseableProblem) {
	for _, pp := range p.Subproblems {
		probs = append(probs, pp)
	}
	return
}

func (p parseableProblem) hasCode() bool   { return p.Code != nil }
func (p parseableProblem) hasName() bool   { return p.Name != nil }
func (p parseableProblem) hasPoints() bool { return p.Points != nil }

type parseableAssignment struct {
	Code     *string            `json:"code"`
	Name     *string            `json:"name"`
	Handins  []parseableHandin  `json:"handins"`
	Problems []parseableProblem `json:"problems"`
}

func (p parseableAssignment) code() string { return *p.Code }

func (p parseableAssignment) name() (s string) {
	if p.Name != nil {
		s = *p.Name
	}
	return
}

func (p parseableAssignment) hasCode() bool { return p.Code != nil }
func (p parseableAssignment) hasName() bool { return p.Name != nil }

func ParseAllAssignmentFiles(ctx *Context) ([]*Assignment, error) {
	dir := ctx.CourseAssignmentDir()
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var reterr error
	var asgns []*Assignment
	for _, f := range files {
		path := filepath.Join(dir, f.Name())
		if !config.IgnoreFileAndLog(ctx.Debug.Printf, path) {
			a, err := ParseAssignmentFile(path)
			if err != nil {
				ctx.Error.Printf("could not read assignment config %v: %v\n", path, err)
				if reterr == nil {
					reterr = err
				}
			} else {
				asgns = append(asgns, a)
			}
		}
	}
	return asgns, reterr
}

func ParseAssignmentFileByCode(ctx *Context, code string) (*Assignment, error) {
	dir := ctx.CourseAssignmentDir()
	return ParseAssignmentFile(filepath.Join(dir, code))
}

func ParseAssignmentFile(path string) (*Assignment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	a, err := parseAssignment(f)
	if err != nil {
		return nil, fmt.Errorf("could not parse: %v", err)
	}
	fname := filepath.Base(path)
	if fname != a.Code {
		return nil, fmt.Errorf("file name does not match assignment code (%v)", a.Code)
	}
	return a, nil
}

func ParseAssignment(ctx *Context, code string) (*Assignment, error) {
	path := filepath.Join(ctx.CourseAssignmentDir(), code)
	return ParseAssignmentFile(path)
}

func parseAssignment(r io.Reader) (*Assignment, error) {
	d := json.NewDecoder(r)
	var asgn parseableAssignment
	err := d.Decode(&asgn)
	if err != nil {
		return nil, err
	}
	if err = validateAssignment(asgn); err != nil {
		return nil, err
	}

	a := &Assignment{
		Code: asgn.code(),
		Name: asgn.name(),
	}
	for _, h := range asgn.Handins {
		a.Handins = append(a.Handins, h.toHandin())
	}
	for _, p := range asgn.Problems {
		a.Problems = append(a.Problems, p.toProblem())
	}
	return a, nil
}

func validateAssignment(asgn parseableAssignment) error {
	if asgn.Code == nil {
		return fmt.Errorf("must have code")
	}
	if err := ValidateCode(*asgn.Code); err != nil {
		return fmt.Errorf("bad assignment code %q: %v", *asgn.Code, err)
	}
	if err := validateProblemTree(asgn.Problems); err != nil {
		return err
	}
	return validateHandins(asgn.Handins, asgn.Problems)
}

func validateProblemTree(problems []parseableProblem) error {
	if len(problems) == 0 {
		return fmt.Errorf("must have at least one problem")
	}

	// Check for code validity - valid codes and no duplicates
	seenCodes := make(map[string]bool)
	var walkTree func(problems []parseableProblem) error
	walkTree = func(problems []parseableProblem) error {
		for _, p := range problems {
			if !p.hasCode() {
				return fmt.Errorf("all problems must have codes")
			}
			c := p.code()
			if err := ValidateCode(c); err != nil {
				return fmt.Errorf("bad problem code %q: %v", c, err)
			}
			if seenCodes[c] {
				return fmt.Errorf("duplicate problem code: %v", c)
			}
			seenCodes[c] = true
			if err := walkTree(p.subproblems()); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walkTree(problems); err != nil {
		return err
	}

	// now check that all problems have points and that
	// they add up properly
	//
	// TODO(joshlf): floating point error?
	var walkTreePoints func(problems []parseableProblem) (float64, error)
	walkTreePoints = func(problems []parseableProblem) (float64, error) {
		var sum float64
		for _, p := range problems {
			if !p.hasPoints() {
				return 0, fmt.Errorf("problem %v must have points", p.code())
			}
			sum += p.points()
			if len(p.subproblems()) > 0 {
				subSum, err := walkTreePoints(p.subproblems())
				if err != nil {
					return 0, err
				}
				if subSum != p.points() {
					return 0, fmt.Errorf("problem %v's points value is not equal to the sum of all subproblems' points", p.code())
				}
			}
		}
		return sum, nil
	}
	if _, err := walkTreePoints(problems); err != nil {
		return err
	}
	return nil
}

// assumes problems have already been validated
func validateHandins(handins []parseableHandin, problems []parseableProblem) error {
	if len(handins) == 0 {
		return fmt.Errorf("must have at least one handin")
	}

	topLevelProblemsByCode := make(map[string]parseableProblem)
	for _, p := range problems {
		topLevelProblemsByCode[p.code()] = p
	}
	problemsByCode := make(map[string]parseableProblem)
	var walkProblems func(problems []parseableProblem)
	walkProblems = func(problems []parseableProblem) {
		for _, p := range problems {
			problemsByCode[p.code()] = p
			walkProblems(p.subproblems())
		}
	}
	walkProblems(problems)

	seenHandinCodes := make(map[string]bool)
	// maps problems to the handin that included them
	seenProblems := make(map[string]string)
	for _, h := range handins {
		switch {
		case len(handins) == 1 && h.hasCode():
			return fmt.Errorf("one handin defined; cannot have handin code")
		case len(handins) > 1 && !h.hasCode():
			return fmt.Errorf("multiple handins defined; each must have a handin code")
		case len(handins) > 1:
			if err := ValidateCode(h.code()); err != nil {
				return fmt.Errorf("bad handin code %q: %v", h.code(), err)
			}
		case !h.hasDue():
			if len(handins) == 1 {
				return fmt.Errorf("handin must have due date")
			}
			return fmt.Errorf("handin %v must have due date", h.code())
		}

		// the name of this handin as it will be printed
		// in error message (either "handin" or "handin <code>")
		handinErrorName := "handin"
		if len(handins) > 1 {
			handinErrorName += " " + string(h.code())
		}
		if len(handins) > 1 && seenHandinCodes[h.code()] {
			return fmt.Errorf("duplicate handin code: %v", h.code())
		}
		seenHandinCodes[h.code()] = true
		if len(h.problems()) == 0 {
			return fmt.Errorf("%v must specify at least one problem", handinErrorName)
		}
		for _, pc := range h.problems() {
			if err := ValidateCode(pc); err != nil {
				return fmt.Errorf("%v contains bad problem code %q: %v", handinErrorName, pc, err)
			}

			_, okTop := topLevelProblemsByCode[pc]
			_, okAll := problemsByCode[pc]
			// okAll implies okTop (topLevelProblemsByCode
			// is a subset of problemsByCode)
			if !okAll {
				return fmt.Errorf("%v specifies nonexistent problem: %v", handinErrorName, pc)
			}
			if !okTop {
				return fmt.Errorf("%v specifies non-top-level problem %v", handinErrorName, pc)
			}
			if hc, ok := seenProblems[pc]; ok {
				return fmt.Errorf("%v includes problem %v, which is also included by handin %v", handinErrorName, pc, hc)
			}
			seenProblems[pc] = h.code()
		}
	}

	// Make sure to traverse problems in order so that
	// we encounter errors in order
	for _, p := range problems {
		if _, ok := seenProblems[p.code()]; !ok {
			return fmt.Errorf("problem %v not in any handins", p.code())
		}
	}
	return nil
}
