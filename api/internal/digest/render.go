package digest

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// disclaimer is the same "awareness tool, not an alert system" notice shown on
// the site — repeated in every digest so the email never reads as an official
// emergency alert.
const disclaimer = "VigilAfrica is an awareness and visualization tool, not an official emergency alert system. " +
	"Event locations and timing may be approximate. Always confirm with local authorities and official " +
	"emergency agencies before making safety decisions."

// digestHTMLTmpl renders the grouped digest. html/template escapes Title in
// text context and sanitizes SourceURL in the href URL context, so event data
// (which originates from an upstream feed) cannot inject markup or scripts.
var digestHTMLTmpl = template.Must(template.New("digest").Parse(`<div style="font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#111;line-height:1.5;">
  <h2 style="margin:0 0 4px;">Daily Flood Digest</h2>
  <p style="margin:0 0 16px;color:#444;">{{.Date}} (UTC) — {{.Total}} flood event{{if ne .Total 1}}s{{end}}.</p>
  {{- if .ByCountry}}
  {{- range .ByCountry}}
  <h3 style="margin:16px 0 4px;">{{.CountryName}}</h3>
    {{- range .States}}
  <p style="margin:8px 0 2px;"><strong>{{.StateName}}</strong></p>
  <ul style="margin:0 0 8px;padding-left:20px;">
    {{- range .Events}}
    <li>{{.Title}}{{if .SourceURL}} — <a href="{{.SourceURL}}">source</a>{{end}}</li>
    {{- end}}
  </ul>
    {{- end}}
  {{- end}}
  {{- else}}
  <p>No flood events recorded today.</p>
  {{- end}}
  <hr style="margin:20px 0 8px;border:none;border-top:1px solid #ddd;">
  <p style="font-size:12px;color:#666;">{{.Disclaimer}}</p>
</div>
`))

// renderDigest produces the HTML and plain-text bodies for the email.
func renderDigest(d Digest) (htmlBody, textBody string, err error) {
	var html strings.Builder
	data := struct {
		Digest
		Disclaimer string
	}{Digest: d, Disclaimer: disclaimer}
	if err := digestHTMLTmpl.Execute(&html, data); err != nil {
		return "", "", fmt.Errorf("render digest html: %w", err)
	}
	return html.String(), renderDigestText(d), nil
}

// renderDigestText builds the plain-text alternative. Built by hand (not a
// template) so there is no HTML-escaping of titles/URLs in the text part.
func renderDigestText(d Digest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Daily Flood Digest\n%s (UTC) — %d flood event%s.\n", d.Date, d.Total, plural(d.Total))

	if len(d.ByCountry) == 0 {
		b.WriteString("\nNo flood events recorded today.\n")
	} else {
		for _, c := range d.ByCountry {
			fmt.Fprintf(&b, "\n%s\n", c.CountryName)
			for _, s := range c.States {
				fmt.Fprintf(&b, "  %s\n", s.StateName)
				for _, e := range s.Events {
					line := "    - " + e.Title
					if e.EventDate != nil {
						line += " (" + e.EventDate.UTC().Format(time.RFC3339) + ")"
					}
					if e.SourceURL != nil && *e.SourceURL != "" {
						line += " — " + *e.SourceURL
					}
					b.WriteString(line + "\n")
				}
			}
		}
	}

	b.WriteString("\n" + disclaimer + "\n")
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
