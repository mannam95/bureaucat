package notifier

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// The HTML side of notification emails, adapted from upstream's digest
// template to our immediate, single-event delivery. Email-client safe:
// tables, bgcolor and inline padding (no margin/display); rounded corners
// degrade to square. The <style> block only tightens spacing on phones;
// clients that drop it get the desktop layout. Every email also carries a
// plain-text alternative (see renderEmail), so text-only clients lose nothing.

const (
	emailFontSans = `-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif`
	emailFontMono = `ui-monospace,SFMono-Regular,Menlo,Consolas,monospace`
)

var emailHTML = template.Must(template.New("email").Funcs(template.FuncMap{
	"sans": func() template.CSS { return emailFontSans },
	"mono": func() template.CSS { return emailFontMono },
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>{{.TaskKey}} {{.TaskTitle}}</title>
<style>
@media only screen and (max-width:600px) {
  .outer { padding:0 !important; }
  .card { border-radius:0 !important; }
  .px { padding-left:16px !important; padding-right:16px !important; }
  .title { font-size:20px !important; line-height:28px !important; }
}
</style>
</head>
<body>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" bgcolor="#f4f4f5">
<tr><td class="outer" align="center" style="padding:32px 12px">
<table class="card" role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" bgcolor="#ffffff" style="max-width:580px;border-radius:12px">

<tr><td class="px" style="padding:16px 28px">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>
<td valign="middle" style="font-family:{{sans}};font-size:17px;line-height:26px;color:#09090b"><b>Bureau<span style="color:#f59e0b">Cat</span></b></td>
<td align="right" valign="middle" style="font-family:{{mono}};font-size:12px;line-height:26px;color:#71717a">{{.ProjectKey}}</td>
</tr></table>
</td></tr>
<tr><td height="1" bgcolor="#e4e4e7" style="font-size:0;line-height:0">&nbsp;</td></tr>

<tr><td class="px" style="padding:24px 28px 0;font-family:{{mono}};font-size:12px;line-height:18px;color:#d97706"><b>{{.TaskKey}}</b></td></tr>
<tr><td class="px title" style="padding:4px 28px 0;font-family:{{sans}};font-size:22px;line-height:30px;color:#09090b"><b>{{.TaskTitle}}</b></td></tr>

<tr><td class="px" style="padding:22px 28px 0"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td height="1" bgcolor="#e4e4e7" style="font-size:0;line-height:0">&nbsp;</td></tr></table></td></tr>

<tr><td class="px" style="padding:16px 28px 0">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
<tr>
<td width="26" valign="top">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
<td width="26" height="26" align="center" valign="middle" bgcolor="#fef3c7" style="border-radius:13px;font-family:{{mono}};font-size:10px;line-height:26px;color:#92400e"><b>{{.Initials}}</b></td>
</tr></table>
</td>
<td valign="middle" style="padding-left:10px;font-family:{{sans}};font-size:14px;line-height:20px;color:#52525b">
<b style="color:#09090b">{{.Actor}}</b> {{.Verb}}
</td>
</tr>
</table>
</td></tr>

<tr><td class="px" style="padding:24px 28px 28px">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
<td bgcolor="#18181b" style="padding:10px 18px;border-radius:8px;font-family:{{sans}};font-size:14px;line-height:20px"><a href="{{.Link}}" style="color:#fafafa;text-decoration:none"><b>View task&nbsp;&rarr;</b></a></td>
</tr></table>
</td></tr>

<tr><td height="1" bgcolor="#e4e4e7" style="font-size:0;line-height:0">&nbsp;</td></tr>
<tr><td class="px" style="padding:16px 28px 20px;font-family:{{sans}};font-size:12px;line-height:18px;color:#71717a">
You are receiving this because you are involved with this task. <a href="{{.SettingsLink}}" style="color:#71717a">Manage email notifications</a>
<br><span style="font-family:{{mono}};font-size:11px;line-height:24px;color:#a1a1aa">Bureaucracy that actually <span style="color:#d97706">moves</span>.</span>
</td></tr>

</table>
</td></tr>
</table>
</body>
</html>`))

type emailHTMLData struct {
	ProjectKey   string
	TaskKey      string
	TaskTitle    string
	Actor        string
	Initials     string
	Verb         string
	Link         string
	SettingsLink string
}

// emailVerb is the per-event sentence rendered after the actor's name,
// mirroring the plain-text copy in renderEmail.
func emailVerb(n Notification) string {
	switch n.Event {
	case EventTaskAssigned:
		return "assigned you to this task"
	case EventMentioned:
		return "mentioned you on this task"
	case EventCommented:
		return "commented on this task"
	case EventStateChanged:
		return "changed the status of this task"
	case EventAddedAsRequester:
		return "added you as a requester"
	case EventAddedAsWatcher:
		return "added you as a watcher"
	case EventActivity:
		return "updated this task"
	default:
		return "updated this task"
	}
}

// actorInitials derives up to two initials from the actor's display name.
func actorInitials(name string) string {
	initials := []rune{}
	for _, word := range strings.Fields(name) {
		initials = append(initials, []rune(word)[0])
		if len(initials) == 2 {
			break
		}
	}
	if len(initials) == 0 {
		return "?"
	}
	return strings.ToUpper(string(initials))
}

// renderEmailHTML builds the HTML body for a notification. An error falls back
// to plain text only (the caller treats an empty string as "no HTML part").
func renderEmailHTML(n Notification) string {
	data := emailHTMLData{
		ProjectKey:   n.ProjectKey,
		TaskKey:      fmt.Sprintf("%s-%d", n.ProjectKey, n.TaskNumber),
		TaskTitle:    n.TaskTitle,
		Actor:        n.ActorName,
		Initials:     actorInitials(n.ActorName),
		Verb:         emailVerb(n),
		Link:         n.TaskURL(),
		SettingsLink: n.BaseURL + "/settings",
	}
	var buf bytes.Buffer
	if err := emailHTML.Execute(&buf, data); err != nil {
		return ""
	}
	return buf.String()
}
