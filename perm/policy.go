package perm

// Policy represents a policy which may be nominated by objects under access
// control. It can authorize or deny an action based on the actor (permission
// set), object (if specified) and the verb being performed.
type Policy interface {
	// Returns true iff the policy allows the given verb to be carried out given
	// the given PermissionSet and object. obj may be nil. The meaning of this
	// depends on the policy.
	AllowsVerbObj(verb string, ps PermissionSet, obj interface{}) bool
}

// An object which can reference a security policy.
type Object interface {
	// Return the security policy for this object or nil if no security policy
	// is assigned.
	PermPolicy() Policy
}

// An object which may have an owner.
type Ownable interface {
	// Returns the owner ID. The owner ID should have a sensible string form.
	//
	// Return (nil, false) if no owner ID is assigned.
	PermOwner() (ownerID interface{}, ok bool)
}

// Tests whether a permission set allows a given verb to be performed on a
// given object.
//
// If the object can nominate a security policy, that policy is used.
// Otherwise, condition VERB(1) is checked for, where VERB is the verb name
// given.
func (ps PermissionSet) AllowsVerbObj(verb string, obj Object) bool {
	if obj == nil {
		return ps.Has(verb, 1)
	}

	policy := obj.PermPolicy()
	if policy == nil {
		return ps.Has(verb, 1)
	}

	return policy.AllowsVerbObj(verb, ps, obj)
}
