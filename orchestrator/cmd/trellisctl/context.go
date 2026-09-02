package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewContextCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "context", Short: "Manage named cluster contexts", Long: "Save and select named cluster connections so normal Trellis commands do not need repeated address, token, namespace, or TLS flags."}
	cmd.AddCommand(newContextListCmd(), newContextCurrentCmd(), newContextShowCmd(), newContextSaveCmd(), newContextUseCmd(), newContextDeleteCmd())
	return cmd
}

func newContextListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List saved contexts", RunE: func(cmd *cobra.Command, _ []string) error {
		path, err := userConfigPath(); if err != nil { return err }
		file, err := readUserConfig(path)
		if os.IsNotExist(err) { _, err = fmt.Fprintln(cmd.OutOrStdout(), "No saved contexts"); return err }
		if err != nil { return err }
		if len(file.Contexts) == 0 { _, err = fmt.Fprintln(cmd.OutOrStdout(), "No saved contexts"); return err }
		names := make([]string, 0, len(file.Contexts)); for name := range file.Contexts { names = append(names, name) }; sort.Strings(names)
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "Current\tName\tAddress\tNamespace\tTLS"); err != nil { return err }
		for _, name := range names {
			ctx := file.Contexts[name]; current := ""; if name == file.CurrentContext { current = "*" }
			ns := ctx.Namespace; if ns == "" { ns = "(cluster)" }
			tlsState := "no"; if ctx.CACert != "" || ctx.Cert != "" || ctx.Key != "" { tlsState = "yes" }
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", current, name, ctx.ServerAddr, ns, tlsState); err != nil { return err }
		}
		return w.Flush()
	}}
}

func newContextCurrentCmd() *cobra.Command {
	return &cobra.Command{Use: "current", Short: "Show the active context and effective connection", RunE: func(cmd *cobra.Command, _ []string) error {
		name := config.Context; if name == "" { name = "(none)" }
		ns := config.Namespace; if ns == "" { ns = "(cluster)" }
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Context: %s\nAddress: %s\nNamespace: %s\nToken: %s\n", name, config.ServerAddr, ns, configured(config.ClusterToken)); return err
	}}
}

func newContextShowCmd() *cobra.Command {
	return &cobra.Command{Use: "show [NAME]", Args: cobra.MaximumNArgs(1), Short: "Show a saved context without revealing its token", RunE: func(cmd *cobra.Command, args []string) error {
		name := config.Context; if len(args) == 1 { name = args[0] }; if name == "" { return fmt.Errorf("no context selected; pass a name or use 'trellisctl context use NAME'") }
		path, err := userConfigPath(); if err != nil { return err }; file, err := readUserConfig(path); if err != nil { return err }
		ctx, ok := file.Contexts[name]; if !ok { return fmt.Errorf("context %q not found", name) }
		ns := ctx.Namespace; if ns == "" { ns = "(cluster)" }
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\nAddress: %s\nNamespace: %s\nToken: %s\nCA: %s\nClient certificate: %s\n", name, ctx.ServerAddr, ns, configured(ctx.ClusterToken), configured(ctx.CACert), configured(ctx.Cert)); return err
	}}
}

func newContextSaveCmd() *cobra.Command {
	var selectContext bool
	cmd := &cobra.Command{Use: "save NAME", Args: cobra.ExactArgs(1), Short: "Save the effective connection as a named context", Long: "Save the currently effective address, token, namespace, and TLS configuration. For example:\n\n  trellisctl --server-addr cluster.example:8128 --cluster-token TOKEN --namespace default context save production --use", RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]; if err := validateContextName(name); err != nil { return err }
		path, err := userConfigPath(); if err != nil { return err }
		file, err := readUserConfig(path); if os.IsNotExist(err) { file = fileConfig{} } else if err != nil { return err }
		if file.Contexts == nil { file.Contexts = make(map[string]contextFileConfig) }
		ctx, err := effectiveContextFileConfig(); if err != nil { return err }; file.Contexts[name] = ctx
		if selectContext { file.CurrentContext = name }
		if err := writeUserConfig(path, file); err != nil { return err }
		if selectContext { _, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved and selected context %s.\n", name) } else { _, err = fmt.Fprintf(cmd.OutOrStdout(), "Saved context %s. Select it with 'trellisctl context use %s'.\n", name, name) }
		return err
	}}
	cmd.Flags().BoolVar(&selectContext, "use", false, "Select this context after saving it")
	return cmd
}

func newContextUseCmd() *cobra.Command {
	return &cobra.Command{Use: "use NAME", Args: cobra.ExactArgs(1), Short: "Select the default context", RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]; path, err := userConfigPath(); if err != nil { return err }; file, err := readUserConfig(path); if err != nil { return err }
		if _, ok := file.Contexts[name]; !ok { return fmt.Errorf("context %q not found", name) }; file.CurrentContext = name
		if err := writeUserConfig(path, file); err != nil { return err }; _, err = fmt.Fprintf(cmd.OutOrStdout(), "Using context %s.\n", name); return err
	}}
}

func newContextDeleteCmd() *cobra.Command {
	return &cobra.Command{Use: "delete NAME", Args: cobra.ExactArgs(1), Short: "Delete a saved context", RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]; path, err := userConfigPath(); if err != nil { return err }; file, err := readUserConfig(path); if err != nil { return err }
		if _, ok := file.Contexts[name]; !ok { return fmt.Errorf("context %q not found", name) }; delete(file.Contexts, name); if file.CurrentContext == name { file.CurrentContext = "" }
		if err := writeUserConfig(path, file); err != nil { return err }; _, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted context %s.\n", name); return err
	}}
}

func configured(value string) string { if value == "" { return "not configured" }; return "configured" }

func validateContextName(name string) error {
	if name == "" { return fmt.Errorf("context name is required") }
	for _, r := range name { if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' { continue }; return fmt.Errorf("context name %q contains unsupported character %q", name, r) }
	return nil
}

func userConfigPath() (string, error) {
	dir, err := os.UserConfigDir(); if err != nil { return "", fmt.Errorf("resolve user config directory: %w", err) }
	return filepath.Join(dir, "trellis", "config.yaml"), nil
}

func readUserConfig(path string) (fileConfig, error) {
	data, err := os.ReadFile(path); if err != nil { return fileConfig{}, err }
	var file fileConfig; if err := yaml.Unmarshal(data, &file); err != nil { return fileConfig{}, fmt.Errorf("parse config file: %w", err) }; return file, nil
}

func writeUserConfig(path string, file fileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
	data, err := yaml.Marshal(&file); if err != nil { return err }
	tmp := path + ".tmp"; if err := os.WriteFile(tmp, data, 0o600); err != nil { return err }
	if err := os.Rename(tmp, path); err != nil { _ = os.Remove(tmp); return err }; return os.Chmod(path, 0o600)
}

func effectiveContextFileConfig() (contextFileConfig, error) {
	var ca string
	if config.CACert != "" { data, err := os.ReadFile(config.CACert); if err != nil { return contextFileConfig{}, fmt.Errorf("read CA cert: %w", err) }; ca = string(data) } else { ca = config.CACertPEM }
	return contextFileConfig{ServerAddr: config.ServerAddr, ClusterToken: config.ClusterToken, Namespace: config.Namespace, CACert: ca, Cert: config.Cert, Key: config.Key}, nil
}
