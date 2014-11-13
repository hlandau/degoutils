package sort

import "container/list"

// © 2001 Simon Tatham    MIT License
// © 2010 Dave Gamble     MIT License
// NO: de:list-sort.c

// Must return true iff a is less than b.
type SortLtFunc func(a, b *list.Element) bool

// Sort a linked list. This is a stable sort.
func SortList(List *list.List, f SortLtFunc) {
	var L, R, N, A, T *list.Element
	var sz uint = 1
	var numMerges, Lsz, Rsz uint

	if List.Len() < 2 {
		return
	}

	A = List.Front()
	//Z = List.Back()   (not needed)

	// STABLE
	for {
		// 'A' is either the head of the original list (on the first iteration), or
		// a list whose subdivisions of 2*(sz/2) are sorted. Either way, 'A' is now
		// 'L' and 'A' and 'T' will be used to assemble the next 'generation' of
		// the sorted list.
		numMerges = 0
		L = A
		A = nil
		T = A

		for L != nil {
			numMerges++
			R = L
			Lsz = 0
			Rsz = sz

			// Step R so that we have adjacent but non-overlapping partitions
			//   [L,R)          (range Lsz)
			// and
			//   [R,R+Rsz)      (range Rsz)
			// The union of which covers completely the subsection of the list
			//   [L,R+Rsz)      (range of up to 2*sz)
			// which is the part of the list which we are sorting at the moment.
			//
			// After this Lsz == Rsz and our partitions are of equal size, unless !R
			// in which case the partitions may be unequal.
			for R != nil && Lsz < sz {
				Lsz++
				R = R.Next()
			}

			//   MERGE
			// So, as long as either the L-list is non-empty (Lsz > 0)
			// or the R-list is non-empty (Rsz > 0 and R points to something
			// non-NULL), we have things to merge in our biparted list subsection.
			//
			for Lsz > 0 || (Rsz > 0 && R != nil) {
				// Choose which list to take the next element from.
				// If either list is empty, we must choose from the other one.
				//
				// In any case, the list taken from is moved up and its size
				// decremented.
				//
				// N identifies the new item to be added to the sorted list.
				//
				if Lsz == 0 {
					// The L-side is empty; choose from R.
					N = R
					R = R.Next()
					Rsz--
				} else if Rsz == 0 || R == nil {
					// The R-side is empty; choose from L.
					N = L
					L = L.Next()
					Lsz--
				} else if f(R, L) {
					// If both lists are non-empty, compare the first element of each and
					// choose the lower one.
					N = R
					R = R.Next()
					Rsz--
				} else {
					// "If the first elements compare equal, choose from the L-side."
					N = L
					L = L.Next()
					Lsz--
				}

				if T != nil {
					// "NEXT(T) = N"
					List.MoveAfter(N, T)
				} else {
					A = N
				}

				// "PREV(N) = T"; this is taken care of by MoveAfter

				// N is now the tail.
				T = N
			}

			// Now A (with tail T) is a completely sorted list of the sublist.
			// We have advanced L until it is where R started out, and we have
			// advanced R until it is pointing at the next pair of length-K lists to
			// merge. So set L to the value of R, and go back to the start of this
			// loop.
			L = R
		}

		// Now we have gone through the entire list forming biparted sublists of
		// sizes (up to) 2*sz. It is time to move "up" by doubling sz and merging
		// again.
		//
		// We keep track of Z so we can set//tail at the end, if necessary.
		//
		// Ensure the tail's next pointer is properly NULLed.

		//NEXT(T)=nil
		//Z = T     (not needed)
		sz = sz * 2

		if numMerges <= 1 {
			break
		}
	}
}
