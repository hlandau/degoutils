// Package perm provides authorization testing functions based on permission sets.
package perm

import "fmt"
import "strings"
import "encoding/gob"

func init() {
	gob.RegisterName("perm.Permission", Permission{})
	gob.RegisterName("perm.PermissionSet", PermissionSet{})
	gob.RegisterName("perm.Implication", Implication{})
	gob.RegisterName("perm.ImplicationSet", ImplicationSet{})
	gob.RegisterName("perm.Condition", Condition{})
}

// A Permission is a label which may be possessed by some manner of actor.
// The authorization of a user to take actions depends on the Permissions they have.
//
// Each permission is comprised of a name and a level, written as "name(level)".
// For example, Permission{ Name: "admin", Level: 10, } can be written in this
// shorthand as "admin(10)".
//
// Stating that an actor must have the permission "foo(5)" means that their
// foo permission must exist and have a level greater than or equal to 5.
//
// An actor is considered to have all permissions they don't have as level 0.
// The default level for a permission an actor is granted is generally level 1.
// Thus for most permissions, checking whether a permission is positive is an
// effective way of checking whether an actor has that permission.
//
// Do not make "negative" permissions like "banned". Instead, make a positive
// permission like "can-access" with a negative level. Then "can-access(0)" is
// necessary to access a service. Explicitly assign banned actors "can-access(-1)".
//
// Permission names should conventionally be formed using lowercase letters, numbers
// and hyphens, not underscores. The actually permitted set of characters is larger than this;
// underscores, dots and colons characters are allowed.
type Permission struct {
	// The permission name.
	Name string

	// The permission level. Aabsent permissions default to level 0, present
	// permissions default to level 1.
	Level int
}

// Returns a string in the form "name(level)".
func (p *Permission) String() string {
	return fmt.Sprintf("%s(%d)", p.Name, p.Level)
}

// A Condition represents some sort of predicate applied to an actor's
// permission set.
//
// While it currently has the same fields as Permission, it is a different
// structure because the semantics of the fields are different; MinLevel is
// greater-than-or-equal-to matched to the Level of a Permission.
type Condition struct {
	// The permission which the condition requires.
	Name string

	// The minimum level of the permission (greater than or equal to).
	MinLevel int
}

// Returns a string in the form "name(min-level)".
func (c *Condition) String() string {
	return fmt.Sprintf("%s(%d)", c.Name, c.MinLevel)
}

// A permission set is a set of permissions held by an actor.
//
// (The invariant s[k].Name == k for all k in permission set s must be
// maintained at all times.)
type PermissionSet map[string]Permission

// Returns true iff the condition is met.
func (ps PermissionSet) Meets(c Condition) bool {
	p, ok := ps[c.Name]
	if !ok {
		return c.MinLevel <= 0
	}

	return p.Level >= c.MinLevel
}

// Returns true if the permission set contains a permission with the given
// name and a level greater than or equal to the given level.
//
// Note: If level less than or equal to 0, returns true if the user does not
// have the permission set.
func (ps PermissionSet) Has(name string, minLevel int) bool {
	return ps.Meets(Condition{Name: name, MinLevel: minLevel})
}

// Returns true iff the permission set contains the permission with the given
// name with a positive level.
func (ps PermissionSet) Positive(name string) bool {
	return ps.Has(name, 1)
}

func (ps PermissionSet) String() string {
	var s []string
	for _, p := range ps {
		s = append(s, p.String())
	}
	return strings.Join(s, ", ")
}

// Merges a permission into a permission set. If the permission does not exist
// in the set, it is added. If the permission already exists, its level will
// be raised if the new permission has a higher level; otherwise, nothing is
// changed.
func (ps PermissionSet) Merge(permission Permission) {
	p, ok := ps[permission.Name]
	if !ok {
		ps[permission.Name] = permission
		return
	}

	if permission.Level > p.Level {
		p.Level = permission.Level
	}
}

// Conditionally apply a given implication.
func (ps PermissionSet) ApplyImplication(impl Implication) {
	if !ps.Meets(impl.Condition) {
		return
	}

	ps.Merge(impl.ImpliedPermission)
}

// Apply a set of implications.
func (ps PermissionSet) ApplyImplications(is ImplicationSet) {
	for i := range is {
		ps.ApplyImplication(is[i])
	}
}

// Makes a copy of the permission set.
func (ps PermissionSet) Copy() PermissionSet {
	ps2 := PermissionSet{}
	for k, v := range ps {
		ps2[k] = v
	}
	return ps2
}

// An implication represents a permission which can be implied by a condition
// (that is, by another permission).
//
// If the Condition is met, ImpliedPermission is implied. The ImpliedPermission will be
// merged with the PermissionSet, which means that ImpliedPermission will only raise the
// level of that permission, not lower it.
type Implication struct {
	// The Condition which must be met in order for the Implication to apply.
	Condition Condition

	// The Permission which is merged with a PermissionSet if the Implication
	// applies.
	ImpliedPermission Permission
}

// Returns a string representation of the implication in the form "condition =>
// implied-permission", e.g. "foo(5) => bar(10)".
func (impl *Implication) String() string {
	return impl.Condition.String() + " => " + impl.ImpliedPermission.String()
}

// A set of implications.
type ImplicationSet []Implication

// Returns a comma separated list of stringized Implications.
func (is ImplicationSet) String() string {
	var s []string
	for _, impl := range is {
		s = append(s, impl.String())
	}
	return strings.Join(s, ", ")
}
