package main

func merge(a, b []string) []string {
	maxOverwrap := min(len(a), len(b))
L_OVERWRAP:
	for o := maxOverwrap; o >= 0; o-- {
		for i := 0; i < o; i++ {
			if b[i] != a[len(a)-o+i] {
				continue L_OVERWRAP
			}
		}
		a = a[0 : len(a)-o]
		break
	}
	a = append(a, b...)
	return a
}
