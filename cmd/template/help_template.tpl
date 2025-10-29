{{- with (or .Long .Short)}}{{.}}{{end}}

Usage:
{{- if .Runnable}}
  {{.UseLine}}
{{- else if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]
{{- end}}

{{- /* ============================================ */}}
{{- /* Available Commands Section                 */}}
{{- /* ============================================ */}}
{{- if .HasAvailableSubCommands}}

Available Commands:
  {{- $groupsUsed := false -}}
  {{- $firstGroup := true -}}

  {{- range $grp := .Groups}}
    {{- $has := false -}}
    {{- range $.Commands}}
      {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID $grp.ID))}}
        {{- $has = true}}
      {{- end}}
    {{- end}}
    
    {{- if $has}}
      {{- $groupsUsed = true -}}
      {{- if $firstGroup}}{{- $firstGroup = false -}}{{else}}

{{- end}}

  {{printf "%s:" $grp.Title}}
      {{- range $.Commands}}
        {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID $grp.ID))}}
    {{rpad .Name .NamePadding}}  {{.Short}}
        {{- end}}
      {{- end}}
    {{- end}}
  {{- end}}

  {{- if $groupsUsed }}
    {{- /* Groups are in use; show ungrouped as "Other" if any */}}
    {{- if hasUngrouped .}}

  Other:
      {{- range .Commands}}
        {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID ""))}}
    {{rpad .Name .NamePadding}}  {{.Short}}
        {{- end}}
      {{- end}}
    {{- end}}
  {{- else }}
    {{- /* No groups at this level; show a flat list with no "Other" header */}}
    {{- range .Commands}}
      {{- if (and (not .Hidden) (.IsAvailableCommand))}}
    {{rpad .Name .NamePadding}}  {{.Short}}
      {{- end}}
    {{- end}}
  {{- end }}
{{- end }}

{{- if .HasExample}}

Examples:
{{.Example}}
{{- end }}

{{- $local := (.LocalFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces) -}}
{{- if $local }}

Flags:
{{$local}}
{{- end }}

{{- $inherited := (.InheritedFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces) -}}
{{- if $inherited }}

Global Flags:
{{$inherited}}
{{- end }}

{{- if .HasAvailableSubCommands }}

Use "{{.CommandPath}} [command] --help" for more information about a command.
{{- end }}

💡 Tip: New here? Run:
  $ cre login
    to login into your cre account, then:
  $ cre init
    to create your first cre project.

📘 Need more help?
  Visit https://docs.chain.link/cre

