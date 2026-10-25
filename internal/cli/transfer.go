package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/transfer"
	"github.com/EdrenaVosh/squadup/internal/tui"
	"github.com/EdrenaVosh/squadup/internal/version"
	"github.com/EdrenaVosh/squadup/internal/workspace"
	"github.com/EdrenaVosh/squadup/templates"
)

const collectionFile = "squadup-presets.squad.json"

func (a *app) generator() string { return "squadup@" + version.Version }

func (a *app) fetcher() transfer.Fetcher {
	return transfer.Fetcher{HTTP: a.io.HTTP, Stdin: a.io.In}
}

func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// ---- export ----

func (a *app) newExport() *cobra.Command {
	var out string
	var all, gist bool
	cmd := &cobra.Command{
		Use:   "export [preset]",
		Short: "Write a preset to a .squad.json file, to stdout, or to a secret gist",
		Long: "Writes one file with the preset and its agents, ready to share. `-o -` prints it to stdout.\n" +
			"Settings other than model, effort and permissions are never included, and neither is `env`.\n" +
			"Before anything leaves, the agents are scanned for strings that look like API keys.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (len(args) == 1) {
				return &ExitError{Code: 2, Err: errors.New("name one preset, or use --all for every preset"), Fix: "`squad export <preset>` or `squad export --all`"}
			}
			bundles, err := a.exportBundles(args, all)
			if err != nil {
				return err
			}
			if err := a.previewExport(bundles); err != nil {
				return err
			}

			var data []byte
			file, desc := bundles[0].Preset.Name+".squad.json", "squadup preset: "+bundles[0].Preset.Name
			if all {
				data, err = transfer.ExportAll(bundles, a.generator(), a.now())
				file, desc = collectionFile, "squadup presets"
			} else {
				data, err = transfer.Export(bundles[0], a.generator(), a.now())
			}
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "fix the agent file with `squad edit`"}
			}

			switch {
			case out == "-":
				_, _ = a.io.Out.Write(data)
			case out != "" || !gist:
				if out == "" {
					out = file
				}
				if err := writeNew(out, data); err != nil {
					return err
				}
				shown := out
				if abs, err := filepath.Abs(out); err == nil {
					shown = abs
				}
				printf(a.io.Out, "Exported %s to %s.\n", exportedWhat(bundles), shown)
			}
			if gist {
				return a.uploadGist(commandContext(cmd), file, desc, data)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "file to write, or - for stdout (default: <preset>.squad.json)")
	cmd.Flags().BoolVar(&all, "all", false, "export every preset into one file")
	cmd.Flags().BoolVar(&gist, "gist", false, "upload to a secret GitHub gist with the gh CLI")
	return cmd
}

func exportedWhat(bundles []preset.Bundle) string {
	if len(bundles) == 1 {
		return fmt.Sprintf("%q", bundles[0].Preset.Name)
	}
	return plural(len(bundles), "preset")
}

func (a *app) exportBundles(args []string, all bool) ([]preset.Bundle, error) {
	store := a.workspace().Store()
	names := args
	if all {
		var err error
		if names, err = store.Names(); err != nil {
			return nil, libraryError(err, "")
		}
		names = slices.DeleteFunc(names, func(n string) bool { return n == "initial" })
		if len(names) == 0 {
			return nil, &ExitError{Code: 1, Err: errors.New("there are no presets to export"), Fix: "create one with `squad new <name>`"}
		}
	}
	var bundles []preset.Bundle
	for _, name := range names {
		if name == "initial" {
			return nil, &ExitError{Code: 1, Err: errors.New("initial is your original setup and may hold private agents, so it cannot be exported directly"), Fix: cloneInitialFix + ", then export the copy"}
		}
		b, err := store.Load(name)
		if err != nil {
			return nil, libraryError(err, "")
		}
		bundles = append(bundles, *b)
	}
	return bundles, nil
}

// previewExport says what is about to leave, and stops on anything that looks
// like a secret until the author agrees.
func (a *app) previewExport(bundles []preset.Bundle) error {
	var secrets int
	for _, b := range bundles {
		d := transfer.Describe(b)
		names := make([]string, len(d.Agents))
		for i, ag := range d.Agents {
			names[i] = ag.Name
		}
		printf(a.io.Err, "Exporting %s: %s (%s); settings: %s. Never included: env and your other settings.\n",
			d.Name, tui.AgentCount(len(names)), strings.Join(names, ", "), d.Settings)
		for _, s := range transfer.ScanSecrets(b) {
			if secrets == 0 {
				printf(a.io.Err, "\nThese look like secret keys:\n")
			}
			secrets++
			printf(a.io.Err, "  %s / %s.md line %d: %s\n", d.Name, s.Agent, s.Line, s.Snippet)
		}
	}
	if secrets == 0 || a.yes {
		return nil
	}
	if !a.io.Interactive {
		return &ExitError{
			Code: 1,
			Err:  errors.New("an agent contains something that looks like a secret key"),
			Fix:  "remove it with `squad edit <preset> <agent>`, or re-run with --yes to export anyway",
		}
	}
	ok, err := a.confirm("\nExport anyway? [y/N] ")
	if err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("read the answer: %w", err)}
	}
	if !ok {
		return a.cancelled()
	}
	return nil
}

func writeNew(file string, data []byte) error {
	if _, err := os.Stat(file); err == nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("%s already exists", file), Fix: "pick another file with -o, or remove it first"}
	}
	if err := fsx.WriteFileAtomic(file, data, 0o644); err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "check that the folder exists and is writable"}
	}
	return nil
}

func (a *app) uploadGist(ctx context.Context, file, desc string, data []byte) error {
	url, err := transfer.CreateGist(ctx, "", file, data, desc)
	if err != nil {
		return transferError(err)
	}
	printf(a.io.Out, "Uploaded to %s\nIt is a secret gist: anyone with the link can read it, nobody finds it by searching.\nImport it elsewhere with `squad import gist:%s`.\n",
		url, path.Base(url))
	return nil
}

// transferError turns a fetch or gh failure into a user error.
func transferError(err error) error {
	switch {
	case errors.Is(err, transfer.ErrNoGH):
		return &ExitError{Code: 1, Err: err, Fix: "install it from https://cli.github.com and run `gh auth login`, or share a file instead"}
	case errors.Is(err, transfer.ErrTooLarge):
		return &ExitError{Code: 1, Err: err, Fix: "a preset file is a few kilobytes; check that this is the right file"}
	}
	return &ExitError{Code: 1, Err: err, Fix: "check the path or link, and that gh is logged in for gists (`gh auth status`)"}
}

// ---- import ----

func (a *app) newImport() *cobra.Command {
	var as string
	var force, allowHooks, acceptRisks bool
	cmd := &cobra.Command{
		Use:   "import <file | https-link | gist:id | ->",
		Short: "Add a shared preset to your library (it is never equipped)",
		Long: "Reads a .squad.json file from a path, an https link, a gist (`gist:<id>`, needs the gh CLI) or stdin (`-`).\n" +
			"The preset is checked and previewed first. It is only stored: `squad use` equips it.\n" +
			"Hooks and MCP servers inside agents are removed unless you pass --allow-hooks.\n" +
			"Settings that switch permission prompts off need --accept-risks (or a yes at the prompt).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := commandContext(cmd)
			source := args[0]

			data, err := a.fetcher().Fetch(ctx, source)
			if err != nil {
				return transferError(err)
			}
			imported, err := transfer.Parse(data, transfer.Options{AllowHooks: allowHooks})
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "ask the sender to export it again with the current squad"}
			}
			if as != "" && len(imported) > 1 {
				return &ExitError{Code: 2, Err: errors.New("--as names one preset, but the file holds several"), Fix: "import without --as, then `squad rename`"}
			}

			dangerous := false
			for _, imp := range imported {
				a.previewImport(imp)
				dangerous = dangerous || imp.Dangerous()
			}
			if dangerous && !acceptRisks {
				if err := a.askRisks(source); err != nil {
					return err
				}
			}

			w := a.workspace()
			for _, imp := range imported {
				res, err := w.Import(imp.Bundle, workspace.ImportOptions{As: as, Force: force})
				if err != nil {
					return libraryError(err, "")
				}
				a.reportImport(imp, res)
				for _, warning := range w.ModelWarnings(ctx, &imp.Bundle) {
					printf(a.io.Out, "Note: %s\n", warning)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "store the preset under this name")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing preset with the same name")
	cmd.Flags().BoolVar(&allowHooks, "allow-hooks", false, "keep the hooks and MCP servers of imported agents")
	cmd.Flags().BoolVar(&acceptRisks, "accept-risks", false, "accept presets that switch permission prompts off")
	return cmd
}

func (a *app) previewImport(imp transfer.Imported) {
	d := transfer.Describe(imp.Bundle)
	printf(a.io.Out, "Preview of %q", d.Name)
	if d.Title != "" && d.Title != d.Name {
		printf(a.io.Out, " (%s)", d.Title)
	}
	printf(a.io.Out, "\n  %s mode", d.Mode)
	if d.Main != "" {
		printf(a.io.Out, " · main agent %s", d.Main)
	}
	printf(a.io.Out, " · %s\n  settings: %s\n", tui.AgentCount(len(d.Agents)), d.Settings)
	for _, ag := range d.Agents {
		printf(a.io.Out, "    %-18s %-8s %-22s %s\n", ag.Name, ag.Model, ag.Tools, ag.Role)
	}
	for _, f := range imp.Findings {
		label := "warn  "
		if f.Level == transfer.LevelDanger {
			label = "DANGER"
		}
		printf(a.io.Out, "  %s  %s\n", label, f.Message)
	}
	printf(a.io.Out, "\n")
}

// askRisks gets the author's consent to import a preset that turns permission
// prompts off. --yes does not count: it is about routine questions only.
func (a *app) askRisks(source string) error {
	if !a.io.Interactive || source == "-" {
		return &ExitError{
			Code: 1,
			Err:  errors.New("the preset switches permission prompts off, and nothing was imported"),
			Fix:  "read the DANGER lines above; if you trust the sender, re-run with --accept-risks",
		}
	}
	ok, err := a.confirm("Import it anyway? [y/N] ")
	if err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("read the answer: %w", err)}
	}
	if !ok {
		return a.cancelled()
	}
	return nil
}

func (a *app) reportImport(imp transfer.Imported, res workspace.ImportResult) {
	count := tui.AgentCount(len(imp.Bundle.Agents))
	switch {
	case res.Replaced:
		printf(a.io.Out, "Replaced %q (%s). It is not equipped.\n", res.Name, count)
	case res.Renamed:
		printf(a.io.Out, "Imported %q as %q (%s). It is not equipped.\n", imp.Bundle.Preset.Name, res.Name, count)
	default:
		printf(a.io.Out, "Imported %q (%s). It is not equipped.\n", res.Name, count)
	}
	printf(a.io.Out, "Check it with `squad lint %s`, equip it with `squad use %s`.\n", res.Name, res.Name)
}

// ---- templates ----

func (a *app) newTemplates() *cobra.Command {
	return &cobra.Command{
		Use:   "templates",
		Short: "List the starter presets that come with squad",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			list, err := templates.List()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "this is a bug in squad; please report it"}
			}
			for _, t := range list {
				printf(a.io.Out, "%-16s %s\n", t.ID, t.Title)
				printf(a.io.Out, "%-16s %s · %s\n", "", t.Description, strings.Join(t.Agents, ", "))
			}
			printf(a.io.Out, "\nStart from one with `squad new --template <id>`.\n")
			return nil
		},
	}
}

// ---- card ----

func (a *app) newCard() *cobra.Command {
	var presetName string
	cmd := &cobra.Command{
		Use:   "card <agent>",
		Short: "Show an agent as a card: role, model, tools and how freely it may act",
		Long:  "Looks in --preset, or else in the equipped preset, or else in the one preset that has the agent.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name, data, err := a.workspace().FindAgent(presetName, args[0])
			var ambiguous *workspace.AmbiguousAgentError
			switch {
			case errors.As(err, &ambiguous):
				return &ExitError{Code: 1, Err: err, Fix: "pick one with --preset"}
			case errors.Is(err, workspace.ErrNoAgent):
				return &ExitError{Code: 1, Err: err, Fix: "`squad lint` shows the agents of each preset"}
			case err != nil:
				return libraryError(err, "")
			}
			agent, err := agentfile.Parse(data)
			if err != nil {
				return &ExitError{Code: 1, Err: fmt.Errorf("%s/%s.md: %w", name, args[0], err), Fix: "fix the file with `squad edit " + name + " " + args[0] + " --raw`"}
			}
			info := tui.AgentCardInfo{Name: args[0], Preset: name, Role: agent.Role()}
			for key, dst := range map[string]*string{
				"color": &info.Color, "model": &info.Model, "effort": &info.Effort, "permissionMode": &info.PermissionMode,
			} {
				if v, ok := agent.Get(key); ok && v != nil {
					*dst = fmt.Sprint(v)
				}
			}
			if v, ok := agent.Get("tools"); ok {
				info.Tools = listText(v)
			}
			printf(a.io.Out, "%s", tui.RenderAgentCard(info, a.useColor()))
			return nil
		},
	}
	cmd.Flags().StringVar(&presetName, "preset", "", "the preset the agent belongs to")
	return cmd
}

// listText renders a frontmatter value that is a string or a list.
func listText(v any) string {
	list, ok := v.([]any)
	if !ok {
		return fmt.Sprint(v)
	}
	parts := make([]string, len(list))
	for i, item := range list {
		parts[i] = fmt.Sprint(item)
	}
	return strings.Join(parts, ", ")
}
