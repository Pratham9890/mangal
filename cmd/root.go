package cmd

import (
	"fmt"
	"github.com/belphemur/mangal/color"
	"github.com/belphemur/mangal/constant"
	"github.com/belphemur/mangal/icon"
	"github.com/belphemur/mangal/key"
	"github.com/belphemur/mangal/log"
	"github.com/belphemur/mangal/packer"
	"github.com/belphemur/mangal/provider"
	"github.com/belphemur/mangal/style"
	"github.com/belphemur/mangal/tui"
	"github.com/belphemur/mangal/util"
	"github.com/belphemur/mangal/version"
	"github.com/belphemur/mangal/where"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"strings"
)

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version")

	rootCmd.PersistentFlags().StringP("format", "F", "", "output format")
	lo.Must0(rootCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return packer.Available(), cobra.ShellCompDirectiveDefault
	}))
	lo.Must0(viper.BindPFlag(key.FormatsUse, rootCmd.PersistentFlags().Lookup("format")))

	rootCmd.PersistentFlags().StringP("icons", "I", "", "icons variant")
	lo.Must0(rootCmd.RegisterFlagCompletionFunc("icons", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return icon.AvailableVariants(), cobra.ShellCompDirectiveDefault
	}))
	lo.Must0(viper.BindPFlag(key.IconsVariant, rootCmd.PersistentFlags().Lookup("icons")))

	rootCmd.PersistentFlags().BoolP("write-history", "H", true, "write history of the read chapters")
	lo.Must0(viper.BindPFlag(key.HistorySaveOnRead, rootCmd.PersistentFlags().Lookup("write-history")))

	rootCmd.PersistentFlags().StringSliceP("source", "S", []string{}, "default source to use")
	lo.Must0(rootCmd.RegisterFlagCompletionFunc("source", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var sources []string

		for _, p := range provider.Builtins() {
			sources = append(sources, p.Name)
		}

		for _, p := range provider.Customs() {
			sources = append(sources, p.Name)
		}

		return sources, cobra.ShellCompDirectiveDefault
	}))
	lo.Must0(viper.BindPFlag(key.DownloaderDefaultSources, rootCmd.PersistentFlags().Lookup("source")))

	rootCmd.Flags().BoolP("continue", "c", false, "continue reading")

	helpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpFunc(cmd, args)
		version.Notify()
	})

	// Clear temporary files on startup
	go func() {
		_ = util.Delete(where.Temp())
	}()
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   constant.Mangal,
	Short: "The ultimate manga downloader",
	Long: constant.AsciiArtLogo + "\n" +
		style.New().Italic(true).Foreground(color.HiRed).Render("    - The ultimate cli manga downloader"),
	PreRun: func(cmd *cobra.Command, args []string) {
		if _, err := packer.Get(viper.GetString(key.FormatsUse)); err != nil {
			handleErr(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flags().Changed("version") {
			versionCmd.Run(versionCmd, args)
			return
		}

		options := tui.Options{
			Continue: lo.Must(cmd.Flags().GetBool("continue")),
		}
		_, err := tui.Run(&options)
		handleErr(err)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if viper.GetBool(key.CliColored) {
		// colored cobra injection
		cc.Init(&cc.Config{
			RootCmd:       rootCmd,
			Headings:      cc.HiCyan + cc.Bold + cc.Underline,
			Commands:      cc.HiYellow + cc.Bold,
			Example:       cc.Italic,
			ExecName:      cc.Bold,
			Flags:         cc.Bold,
			FlagsDataType: cc.Italic + cc.HiBlue,
		})
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func handleErr(err error) {
	if err != nil {
		log.Error(err)
		_, _ = fmt.Fprintf(os.Stderr, "%s %s\n", icon.Get(icon.Fail), strings.Trim(err.Error(), " \n"))
		os.Exit(1)
	}
}
