package sortdeplist

import "sort"

func SortInplace(depList []string) {
	sort.Slice(depList, func(i, j int) bool {
		return depList[i] < depList[j]
	})
}
