package loans

import (
	"html/template"
	"net/http"
	"time"
)

var memberPage = template.Must(template.New("member").Parse(`<!doctype html>
<html>
<head>
<title>{{.Member.Name}}</title>
<style>.badge-overdue { color: #b00020; font-weight: bold; }</style>
</head>
<body>
<h1>{{.Member.Name}}</h1>
<ul>
{{range .Loans}}<li>{{.CopyID}}, due {{.DueAt.Format "2006-01-02"}}{{if and $.Badges .Overdue}} <span class="badge-overdue">Overdue</span>{{end}}</li>
{{end}}</ul>
</body>
</html>
`))

type loanView struct {
	CopyID  string
	DueAt   time.Time
	Overdue bool
}

// MemberPage renders the loans of the member named by the member query
// parameter.
func (s *Service) MemberPage(w http.ResponseWriter, r *http.Request) {
	member, ok := s.members[r.URL.Query().Get("member")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	var views []loanView
	for _, loan := range s.ActiveLoans(member.ID) {
		views = append(views, loanView{CopyID: loan.CopyID, DueAt: loan.DueAt, Overdue: IsOverdue(loan)})
	}
	data := struct {
		Member Member
		Loans  []loanView
		Badges bool
	}{member, views, s.Features.OverdueBadges}
	if err := memberPage.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
