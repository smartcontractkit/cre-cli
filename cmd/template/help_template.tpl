{{- with (or .Long .Short)}}{{.}}{{end}}

{{styleSection "Usage:"}}
{{- if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{if .HasAvailableFlags}} [flags]{{end}}
{{- else}}
  {{.UseLine}}
{{- end}}


{{- /* ============================================ */}}
{{- /* Available Commands Section                 */}}
{{- /* ============================================ */}}
{{- if .HasAvailableSubCommands}}

{{styleSection "Available Commands:"}}
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

  {{styleDim $grp.Title}}
      {{- range $.Commands}}
        {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID $grp.ID))}}
    {{styleCommand (rpad .Name .NamePadding)}}  {{.Short}}
        {{- end}}
      {{- end}}
    {{- end}}
  {{- end}}

  {{- if $groupsUsed }}
    {{- /* Groups are in use; show ungrouped as "Other" if any */}}
    {{- if hasUngrouped .}}

  {{styleDim "Other"}}
      {{- range .Commands}}
        {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID ""))}}
    {{styleCommand (rpad .Name .NamePadding)}}  {{.Short}}
        {{- end}}
      {{- end}}
    {{- end}}
  {{- else }}
    {{- /* No groups at this level; show a flat list with no "Other" header */}}
    {{- range .Commands}}
      {{- if (and (not .Hidden) (.IsAvailableCommand))}}
    {{styleCommand (rpad .Name .NamePadding)}}  {{.Short}}
      {{- end}}
    {{- end}}
  {{- end }}
{{- end }}

{{- if .HasExample}}

{{styleSection "Examples:"}}
{{styleCode .Example}}
{{- end }}

{{- $local := (.LocalFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces) -}}
{{- if $local }}

{{styleSection "Flags:"}}
{{$local}}
{{- end }}

{{- $inherited := (.InheritedFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces) -}}
{{- if $inherited }}

{{styleSection "Global Flags:"}}
{{$inherited}}
{{- end }}

{{- if .HasAvailableSubCommands }}

{{styleDim (printf "Use \"%s [command] --help\" for more information about a command." .CommandPath)}}
{{- end }}

{{styleSuccess "Tip:"}} New here? Run:
  {{styleCode "$ cre login"}}
    to login into your cre account, then:
  {{styleCode "$ cre init"}}
    to create your first cre project.

{{styleSection "Need more help?"}}
  Visit {{styleCommand "https://docs.chain.link/cre"}}

