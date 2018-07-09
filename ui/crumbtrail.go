package ui

import "strings"

type CrumbTrail struct {
	crumbs []string
}

func (ct *CrumbTrail)Add(c string) {
	ct.crumbs = append(ct.crumbs, c)
}

func (ct CrumbTrail)String() string {
	return "{" + strings.Join(ct.crumbs, ",") + "}"
}
