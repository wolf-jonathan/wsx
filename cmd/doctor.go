package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jwolf/wsx/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	doctorStatusOK    = "ok"
	doctorStatusWarn  = "warn"
	doctorStatusError = "error"
)

var doctorIsTerminal = func() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

var doctorVariablePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type doctorReport struct {
	Healthy bool          `json:"healthy"`
	Checks  []doctorCheck `json:"checks"`
}

type doctorResolvedRef struct {
	Ref          workspace.Ref
	ResolvedPath string
	ResolveErr   error
}

type doctorCommandError struct {
	message string
}

func (e *doctorCommandError) Error() string {
	return e.message
}

func newDoctorCommand() *cobra.Command {
	var jsonOutput bool
	var fix bool

	command := &cobra.Command{
		Use:   "doctor",
		Short: "Validate workspace health and portability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			isTerminal := doctorIsTerminal()
			if fix && !isTerminal {
				return errors.New("--fix requires an interactive terminal")
			}

			interactive := fix && isTerminal
			progressOutput := cmd.OutOrStdout()
			if jsonOutput {
				progressOutput = cmd.ErrOrStderr()
			}

			report, err := runDoctor(cmd.InOrStdin(), progressOutput, interactive)
			if err != nil {
				return err
			}

			if jsonOutput {
				if err := writeDoctorJSON(cmd, report); err != nil {
					return err
				}
			} else {
				if err := writeDoctorText(cmd, report); err != nil {
					return err
				}
			}

			if !report.Healthy {
				return &doctorCommandError{message: "workspace has one or more health check failures"}
			}

			return nil
		},
	}

	command.Flags().BoolVar(&fix, "fix", false, "Prompt to resolve unresolved variables and write them to .wsx.env")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output doctor checks as JSON")
	return command
}

func runDoctor(input io.Reader, output io.Writer, interactive bool) (doctorReport, error) {
	loaded, err := workspace.LoadConfig("")
	if err != nil {
		return doctorReport{}, err
	}

	env, envExists, err := loadDoctorEnv(loaded.Root)
	if err != nil {
		return doctorReport{}, err
	}

	checks := []doctorCheck{{
		Name:    "config_valid",
		Status:  doctorStatusOK,
		Message: fmt.Sprintf("%s found and valid", workspace.ConfigFileName),
	}}

	envSaved := false
	varNames := collectDoctorVariables(loaded.Config.Refs)
	if len(varNames) > 0 {
		if err := resolveDoctorVariables(loaded.Root, input, output, env, varNames, interactive, &checks, &envSaved); err != nil {
			return doctorReport{}, err
		}
	}

	switch {
	case envExists || envSaved:
		message := fmt.Sprintf("%s found", workspace.EnvFileName)
		if envSaved && !envExists {
			message = fmt.Sprintf("%s created", workspace.EnvFileName)
		}
		checks = append(checks, doctorCheck{Name: "env_file", Status: doctorStatusOK, Message: message})
	default:
		checks = append(checks, doctorCheck{Name: "env_file", Status: doctorStatusWarn, Message: fmt.Sprintf("%s is missing", workspace.EnvFileName)})
	}

	resolvedRefs := resolveDoctorRefs(loaded.Config.Refs, env)
	checks = append(checks, checkDoctorNameConflicts(loaded.Config.Refs)...)
	checks = append(checks, checkDoctorCaseCollisions(loaded.Config.Refs)...)
	checks = append(checks, checkDoctorWorkspaceNesting(loaded.Root, resolvedRefs)...)
	checks = append(checks, checkDoctorNestedRefs(resolvedRefs)...)
	checks = append(checks, checkDoctorLinks(loaded.Root, resolvedRefs)...)
	checks = append(checks, checkDoctorGitRepos(resolvedRefs)...)

	report := doctorReport{
		Healthy: true,
		Checks:  checks,
	}

	for _, check := range checks {
		if check.Status == doctorStatusError {
			report.Healthy = false
			break
		}
	}

	return report, nil
}

func loadDoctorEnv(root string) (workspace.EnvVars, bool, error) {
	env, err := workspace.LoadEnv(root)
	if err == nil {
		return env, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return workspace.EnvVars{}, false, nil
	}
	return nil, false, err
}

func collectDoctorVariables(refs []workspace.Ref) []string {
	unique := map[string]struct{}{}
	for _, ref := range refs {
		matches := doctorVariablePattern.FindAllStringSubmatch(ref.Path, -1)
		for _, match := range matches {
			if len(match) != 2 {
				continue
			}
			unique[match[1]] = struct{}{}
		}
	}

	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveDoctorVariables(root string, input io.Reader, output io.Writer, env workspace.EnvVars, varNames []string, interactive bool, checks *[]doctorCheck, envSaved *bool) error {
	reader := bufio.NewReader(input)

	for _, name := range varNames {
		checkName := "var_" + name
		if hasDoctorVariableValue(name, env) {
			*checks = append(*checks, doctorCheck{Name: checkName, Status: doctorStatusOK, Message: fmt.Sprintf("%s resolved", name)})
			continue
		}

		if !interactive {
			*checks = append(*checks, doctorCheck{Name: checkName, Status: doctorStatusError, Message: fmt.Sprintf("%s is not defined in %s or environment", name, workspace.EnvFileName)})
			continue
		}

		if _, err := fmt.Fprintf(output, "Enter the path for %s: ", name); err != nil {
			return err
		}

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			*checks = append(*checks, doctorCheck{Name: checkName, Status: doctorStatusError, Message: fmt.Sprintf("no value provided for %s", name)})
			continue
		}

		env[name] = value
		if err := workspace.SaveEnv(root, env); err != nil {
			return err
		}
		*envSaved = true

		if _, err := fmt.Fprintf(output, "Saved %s to %s\n", name, workspace.EnvFileName); err != nil {
			return err
		}

		*checks = append(*checks, doctorCheck{Name: checkName, Status: doctorStatusOK, Message: fmt.Sprintf("%s resolved", name)})
	}

	return nil
}

func hasDoctorVariableValue(name string, env workspace.EnvVars) bool {
	if env != nil {
		if value, ok := env[name]; ok && strings.TrimSpace(value) != "" {
			return true
		}
	}

	value, ok := os.LookupEnv(name)
	return ok && strings.TrimSpace(value) != ""
}

func resolveDoctorRefs(refs []workspace.Ref, env workspace.EnvVars) []doctorResolvedRef {
	items := make([]doctorResolvedRef, 0, len(refs))
	for _, ref := range refs {
		item := doctorResolvedRef{Ref: ref}
		item.ResolvedPath, item.ResolveErr = workspace.ResolvePath(ref.Path, env)
		items = append(items, item)
	}
	return items
}

func checkDoctorNameConflicts(refs []workspace.Ref) []doctorCheck {
	counts := map[string]int{}
	for _, ref := range refs {
		counts[ref.Name]++
	}

	duplicates := make([]string, 0)
	for name, count := range counts {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}
	sort.Strings(duplicates)

	if len(duplicates) == 0 {
		return []doctorCheck{{Name: "no_duplicate_names", Status: doctorStatusOK, Message: "no duplicate ref names"}}
	}

	return []doctorCheck{{
		Name:    "no_duplicate_names",
		Status:  doctorStatusError,
		Message: "duplicate ref names: " + strings.Join(duplicates, ", "),
	}}
}

func checkDoctorCaseCollisions(refs []workspace.Ref) []doctorCheck {
	byFolded := map[string][]string{}
	for _, ref := range refs {
		byFolded[strings.ToLower(ref.Name)] = append(byFolded[strings.ToLower(ref.Name)], ref.Name)
	}

	collisions := make([]string, 0)
	for _, names := range byFolded {
		if len(names) < 2 {
			continue
		}

		unique := map[string]struct{}{}
		for _, name := range names {
			unique[name] = struct{}{}
		}
		if len(unique) < 2 {
			continue
		}

		var variants []string
		for name := range unique {
			variants = append(variants, name)
		}
		sort.Strings(variants)
		collisions = append(collisions, strings.Join(variants, "/"))
	}
	sort.Strings(collisions)

	if len(collisions) == 0 {
		return []doctorCheck{{Name: "no_case_collisions", Status: doctorStatusOK, Message: "no case-collision issues detected"}}
	}

	return []doctorCheck{{
		Name:    "no_case_collisions",
		Status:  doctorStatusError,
		Message: "case-collision ref names: " + strings.Join(collisions, ", "),
	}}
}

func checkDoctorWorkspaceNesting(root string, refs []doctorResolvedRef) []doctorCheck {
	problems := make([]string, 0)
	for _, ref := range refs {
		if ref.ResolveErr != nil {
			continue
		}

		if samePath(root, ref.ResolvedPath) {
			problems = append(problems, ref.Ref.Name+" points at the workspace root")
			continue
		}
		if _, ok := relativeIfWithin(root, ref.ResolvedPath); ok {
			problems = append(problems, ref.Ref.Name+" is inside the workspace")
			continue
		}
		if _, ok := relativeIfWithin(ref.ResolvedPath, root); ok {
			problems = append(problems, ref.Ref.Name+" contains the workspace")
		}
	}

	if len(problems) == 0 {
		return []doctorCheck{{Name: "no_workspace_nesting", Status: doctorStatusOK, Message: "no refs overlap the workspace root"}}
	}

	sort.Strings(problems)
	return []doctorCheck{{
		Name:    "no_workspace_nesting",
		Status:  doctorStatusError,
		Message: strings.Join(problems, "; "),
	}}
}

func checkDoctorNestedRefs(refs []doctorResolvedRef) []doctorCheck {
	problems := make([]string, 0)

	for i := 0; i < len(refs); i++ {
		if refs[i].ResolveErr != nil {
			continue
		}
		for j := i + 1; j < len(refs); j++ {
			if refs[j].ResolveErr != nil {
				continue
			}

			if _, ok := relativeIfWithin(refs[i].ResolvedPath, refs[j].ResolvedPath); ok {
				problems = append(problems, refs[j].Ref.Name+" nested under "+refs[i].Ref.Name)
				continue
			}
			if _, ok := relativeIfWithin(refs[j].ResolvedPath, refs[i].ResolvedPath); ok {
				problems = append(problems, refs[i].Ref.Name+" nested under "+refs[j].Ref.Name)
			}
		}
	}

	if len(problems) == 0 {
		return []doctorCheck{{Name: "no_nested_refs", Status: doctorStatusOK, Message: "no repos are nested inside other linked repos"}}
	}

	sort.Strings(problems)
	return []doctorCheck{{
		Name:    "no_nested_refs",
		Status:  doctorStatusError,
		Message: strings.Join(problems, "; "),
	}}
}

func checkDoctorLinks(root string, refs []doctorResolvedRef) []doctorCheck {
	checks := make([]doctorCheck, 0, len(refs))
	for _, ref := range refs {
		check := doctorCheck{Name: ref.Ref.Name + "_link"}
		if ref.ResolveErr != nil {
			check.Status = doctorStatusError
			check.Message = ref.ResolveErr.Error()
			checks = append(checks, check)
			continue
		}

		info, err := os.Stat(ref.ResolvedPath)
		if err != nil || !info.IsDir() {
			check.Status = doctorStatusError
			check.Message = fmt.Sprintf("target missing or not a directory: %s", ref.ResolvedPath)
			checks = append(checks, check)
			continue
		}

		linkType, err := validateWorkspaceLinkTarget(root, ref.Ref.Name, ref.ResolvedPath)
		if err != nil {
			check.Status = doctorStatusError
			check.Message = err.Error()
			checks = append(checks, check)
			continue
		}

		check.Status = doctorStatusOK
		check.Message = fmt.Sprintf("%s ok -> %s", linkType, ref.ResolvedPath)
		checks = append(checks, check)
	}

	return checks
}

func checkDoctorGitRepos(refs []doctorResolvedRef) []doctorCheck {
	checks := make([]doctorCheck, 0, len(refs))
	for _, ref := range refs {
		check := doctorCheck{Name: ref.Ref.Name + "_git"}
		if ref.ResolveErr != nil {
			check.Status = doctorStatusWarn
			check.Message = "git check skipped because the ref path could not be resolved"
			checks = append(checks, check)
			continue
		}

		gitPath := filepath.Join(ref.ResolvedPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			check.Status = doctorStatusOK
			check.Message = "git repository detected"
		} else if errors.Is(err, os.ErrNotExist) {
			check.Status = doctorStatusWarn
			check.Message = "linked directory is not a git repository"
		} else {
			check.Status = doctorStatusWarn
			check.Message = "could not verify git repository: " + err.Error()
		}

		checks = append(checks, check)
	}

	return checks
}

func writeDoctorJSON(cmd *cobra.Command, report doctorReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeDoctorText(cmd *cobra.Command, report doctorReport) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", strings.ToUpper(check.Status), check.Name, check.Message); err != nil {
			return err
		}
	}

	return writer.Flush()
}
