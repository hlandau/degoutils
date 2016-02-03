package forms

import (
	"github.com/vincent-petithory/countries"
	"net/http"
	"sort"
)

type countrySorter []*countries.Country

func (cs countrySorter) Len() int {
	return len(cs)
}

func (cs countrySorter) Less(i, j int) bool {
	return cs[i].ISO3166OneAlphaTwo < cs[j].ISO3166OneAlphaTwo
}

func (cs countrySorter) Swap(i, j int) {
	cs[i], cs[j] = cs[j], cs[i]
}

func init() {
	var ctrs []*countries.Country
	for _, v := range countries.Countries {
		ctrs = append(ctrs, &v)
	}

	sort.Sort(countrySorter(ctrs))

	var svs []SelectValue
	for _, c := range ctrs {
		svs = append(svs, SelectValue{
			Value: c.ISO3166OneAlphaTwo,
			Title: c.ISO3166OneEnglishShortNameGazetteerOrder,
		})
	}

	SelectValueFuncs["country"] = func(fi *FieldInfo, req *http.Request) []SelectValue {
		return svs
	}
}
