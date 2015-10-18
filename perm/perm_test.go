package perm

import "testing"
import "reflect"

type test struct {
	In     string
	Out    ImplicationSet
	OutStr string
}

var tests = []test{
	{
		In: `foo(5) => bar(10), baz(1) => abc(2)
    # this is a comment
    q(2) => r(09)`,
		OutStr: `foo(5) => bar(10), baz(1) => abc(2), q(2) => r(9)`,
		Out: ImplicationSet{
			Implication{
				Condition: Condition{
					Name:     "foo",
					MinLevel: 5,
				},
				ImpliedPermission: Permission{
					Name:  "bar",
					Level: 10,
				},
			},

			Implication{
				Condition: Condition{
					Name:     "baz",
					MinLevel: 1,
				},
				ImpliedPermission: Permission{
					Name:  "abc",
					Level: 2,
				},
			},

			Implication{
				Condition: Condition{
					Name:     "q",
					MinLevel: 2,
				},
				ImpliedPermission: Permission{
					Name:  "r",
					Level: 9,
				},
			},
		},
	},
}

func TestParse(t *testing.T) {
	for _, tst := range tests {
		is, err := ParseImplications(tst.In)
		if err != nil {
			t.Fatalf("error parsing implications: %v", err)
		}

		if !reflect.DeepEqual(is, tst.Out) {
			t.Fatalf("did not equal: got %v, expected %v", is, tst.Out)
		}

		s := is.String()
		if s != tst.OutStr {
			t.Fatalf("strings did not equal: got %#v, expected %#v", s, tst.OutStr)
		}
	}
}
