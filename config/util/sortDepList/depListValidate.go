package sortdeplist

import "sort"

func SortInplace(depList []string) {
	sort.Slice(depList, func(i, j int) bool {
		return depList[i] < depList[j]
	})
}

func DepListEq(depList1 []string, depList2 []string) bool {
	SortInplace(depList1)
	SortInplace(depList2)

	if len(depList1) != len(depList2) {
		return false
	}
	for idx1, k1 := range depList1 {
		k2 := depList2[idx1]
		if k1 != k2 {
			return false
		}
	}
	return true
}
