package lib

import (
	"cmp"
	"slices"
)

type XSet[T cmp.Ordered] struct {
	elements []T
}

func NewXSet[T cmp.Ordered](elems []T) XSet[T] {
	slices.Sort(elems)
	return XSet[T]{elements: elems}
}

// the return list is belong xSet, if you want list in ySet, you should call Intersect on ySet
func (xSet XSet[T]) Intersect(ySet XSet[T], isIntersect func(selfElem, argElem T) bool) []T {
	var intersectList []T
	for _, elem1 := range xSet.elements {
		for _, elem2 := range ySet.elements {
			if isIntersect(elem1, elem2) {
				intersectList = append(intersectList, elem1)
				break
			}
		}
	}
	return intersectList
}
