package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

const (
	doctorStatusOK    = "ok"
	doctorStatusWarn  = "warn"
	doctorStatusError = "error"
)

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

	command := &cobra.Command{
		Use:   "doctor",
		Short: "Validate workspace health",
		Args:  cobra.NoArgs,
		Example: `wsx doctor
wsx doctor --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := runDoctor()
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

	command.Flags().BoolVar(&jsonOutput, "json", false, "Output doctor checks as JSON")
	return command
}

func runDoctor() (doctorReport, error) {
	loaded, err := workspace.LoadConfig("")
	if err != nil {
		return doctorReport{}, err
	}

	checks := []doctorCheck{{
		Name:    "config_valid",
		Status:  doctorStatusOK,
		Message: fmt.Sprintf("%s found and valid", workspace.ConfigFileName),
	}}

	resolvedRefs := resolveDoctorRefs(loaded.Config.Refs)
	checks = append(checks, checkDoctorStoredPaths(resolvedRefs)...)
	checks = append(checks, checkDoctorNameConflicts(loaded.Config.Refs)...)
	checks = append(checks, checkDoctorCaseCollisions(loaded.Config.Refs)...)
	checks = append(checks, checkDoctorWorkspaceNesting(loaded.Root, resolvedRefs)...)
	checks = append(checks, checkDoctorNestedRefs(resolvedRefs)...)
	checks = append(checks, checkDoctorLinks(loaded.Root, resolvedRefs)...)
	checks = append(checks, checkDoctorGitRepos(resolvedRefs)...)
	checks = append(checks, checkDoctorWorkspaceInstructionFiles(loaded, resolvedRefs)...)

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

func resolveDoctorRefs(refs []workspace.Ref) []doctorResolvedRef {
	items := make([]doctorResolvedRef, 0, len(refs))
	for _, ref := range refs {
		item := doctorResolvedRef{Ref: ref}
		item.ResolvedPath, item.ResolveErr = workspace.ResolveStoredPath(ref.Path)
		items = append(items, item)
	}
	return items
}

func checkDoctorStoredPaths(refs []doctorResolvedRef) []doctorCheck {
	problems := make([]string, 0)

	for _, ref := range refs {
		if ref.ResolveErr != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", ref.Ref.Name, ref.ResolveErr))
		}
	}

	if len(problems) == 0 {
		return []doctorCheck{{
			Name:    "stored_paths_valid",
			Status:  doctorStatusOK,
			Message: "all ref paths are stored as absolute paths",
		}}
	}

	sort.Strings(problems)
	return []doctorCheck{{
		Name:    "stored_paths_valid",
		Status:  doctorStatusError,
		Message: strings.Join(problems, "; "),
	}}
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
			check.Status = doctorStatusWarn
			check.Message = "link check skipped because the stored path is invalid"
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
			check.Message = "git check skipped because the stored path is invalid"
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

func checkDoctorWorkspaceInstructionFiles(loaded *workspace.LoadedConfig, refs []doctorResolvedRef) []doctorCheck {
	repos := make([]ai.InstructionRepo, 0, len(refs))
	for _, ref := range refs {
		if ref.ResolveErr != nil {
			return nil
		}

		info, err := os.Stat(ref.ResolvedPath)
		if err != nil || !info.IsDir() {
			return nil
		}

		if _, err := validateWorkspaceLinkTarget(loaded.Root, ref.Ref.Name, ref.ResolvedPath); err != nil {
			return nil
		}

		repos = append(repos, ai.InstructionRepo{
			Name: ref.Ref.Name,
			Root: ref.ResolvedPath,
		})
	}

	expected, err := ai.BuildWorkspaceInstructionContent(loaded.Config.Name, "", repos)
	if err != nil {
		return nil
	}
	expected = normalizeDoctorWorkspaceInstructionContent(expected)

	missing := make([]string, 0, 2)
	stale := make([]string, 0, 2)
	for _, filePath := range []string{ai.WorkspaceAgentsFilePath, ai.WorkspaceClaudeFilePath} {
		fullPath := filepath.Join(loaded.Root, filepath.FromSlash(filePath))
		content, err := os.ReadFile(fullPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, filePath)
				continue
			}

			return []doctorCheck{{
				Name:    "workspace_instruction_files",
				Status:  doctorStatusWarn,
				Message: fmt.Sprintf("could not verify %s: %v", filePath, err),
			}}
		}

		actual := normalizeDoctorWorkspaceInstructionContent(string(content))
		if actual != expected {
			stale = append(stale, filePath)
		}
	}

	if len(missing) == 0 && len(stale) == 0 {
		return []doctorCheck{{
			Name:    "workspace_instruction_files",
			Status:  doctorStatusOK,
			Message: "AGENTS.md and CLAUDE.md are up to date",
		}}
	}

	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		sort.Strings(missing)
		parts = append(parts, pluralizeDoctorInstructionFileList(missing, "is missing", "are missing"))
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		parts = append(parts, pluralizeDoctorInstructionFileList(stale, "is stale", "are stale"))
	}

	return []doctorCheck{{
		Name:    "workspace_instruction_files",
		Status:  doctorStatusWarn,
		Message: strings.Join(parts, "; "),
	}}
}

func normalizeDoctorWorkspaceInstructionContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	skipBlankAfterPurpose := false

	for _, line := range lines {
		if strings.HasPrefix(line, "Purpose: ") {
			skipBlankAfterPurpose = true
			continue
		}

		if skipBlankAfterPurpose {
			if line == "" {
				skipBlankAfterPurpose = false
				continue
			}
			skipBlankAfterPurpose = false
		}

		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
}

func pluralizeDoctorInstructionFileList(paths []string, singular, plural string) string {
	if len(paths) == 1 {
		return fmt.Sprintf("%s %s", paths[0], singular)
	}

	return fmt.Sprintf("%s %s", strings.Join(paths, ", "), plural)
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
