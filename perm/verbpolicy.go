package perm

import "fmt"

// A simple policy based on mapping verb names to conditions.
//
// If a verb is not specified in the map, the condition VERB(1) is used,
// where VERB is the verb name.
//
// Also supports ownership semantics: if a verb nominates a condition with name
// "owner", that condition is substituted for a condition
// "user-id:OWNER-ID(1)", where OWNER-ID is string form of the the owner ID of
// the object being accessed. If no object is specified, or the object does not
// express an owner, ownership-based checks fail.
type VerbPolicy struct {
	Verbs map[string]Condition // verb name -> Condition
}

// Returns true iff the policy allows the given verb to be carried out given
// the given PermissionSet.
//
// If an object is given and supports the Ownable interface, and it nominates
// an owner ID, and the verb requires ownership, returns true if the
// "user-id:OWNER-ID(1)" condition is met, where OWNER-ID is the string
// representation of the owner ID. (The idea is that you axiomatically assign
// every user the permission "user-id:USER-ID(1)", where USER-ID is their user
// ID.)
//
// obj may be nil, in which case owner-based checks fail.
func (p *VerbPolicy) AllowsVerbObj(verb string, ps PermissionSet, obj interface{}) bool {
	c, ok := p.Verbs[verb]
	if !ok {
		c = Condition{Name: verb, MinLevel: 1}
	}

	if c.Name == "owner" {
		oo, ok := obj.(Ownable)
		if !ok {
			return false
		}
		ownerID, ok := oo.PermOwner()
		if !ok {
			return false
		}
		c.Name = fmt.Sprintf("user-id:%v", ownerID)
	}

	return ps.Meets(c)
}
