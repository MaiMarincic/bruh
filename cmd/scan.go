package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Check for security issues in AI-generated code",
	Long: `Scan for security vulnerabilities and potential issues in AI-generated code.
Automatically detects project languages and runs appropriate tools:

Multi-language:
- gitleaks: Secret detection in git history
- semgrep: Multi-language static analysis (if available)

Go:
- gosec: Go security checker
- staticcheck: Go static analysis
- govulncheck: Go vulnerability database checker
- go vet: Built-in Go static analysis

JavaScript/TypeScript:
- eslint: JavaScript/TypeScript linting
- npm audit: NPM vulnerability checker

Python:
- bandit: Python security checker
- pylint: Python static analysis
- safety: Python vulnerability checker

Java:
- spotbugs: Java static analysis
- pmd: Java code quality

Rust:
- cargo clippy: Rust linting
- cargo audit: Rust vulnerability checker`,
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		all, _ := cmd.Flags().GetBool("all")
		language, _ := cmd.Flags().GetString("language")

		var secrets, security, static, vulns bool
		if all {
			secrets, security, static, vulns = true, true, true, true
		} else {
			secrets, _ = cmd.Flags().GetBool("secrets")
			security, _ = cmd.Flags().GetBool("security")
			static, _ = cmd.Flags().GetBool("static")
			vulns, _ = cmd.Flags().GetBool("vulns")
		}

		return runMultiLanguageScan(verbose, secrets, security, static, vulns, language)
	},
}

func init() {
	scanCmd.Flags().BoolP("secrets", "s", true, "Scan for committed secrets using gitleaks")
	scanCmd.Flags().BoolP("security", "S", true, "Run language-specific security analysis")
	scanCmd.Flags().BoolP("static", "t", true, "Run language-specific static analysis")
	scanCmd.Flags().BoolP("vulns", "V", true, "Check for known vulnerabilities")
	scanCmd.Flags().BoolP("all", "a", false, "Run all available scans for detected languages")
	scanCmd.Flags().StringP("language", "l", "", "Force specific language (go, js, python, java, rust)")
	scanCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
}

func runMultiLanguageScan(verbose, secrets, security, static, vulns bool, forceLanguage string) error {
	if !secrets && !security && !static && !vulns {
		fmt.Println("No scan types selected. Use --all or enable specific scans.")
		return nil
	}

	languages := []string{}
	if forceLanguage != "" {
		languages = []string{forceLanguage}
	} else {
		languages = detectLanguages()
	}

	if len(languages) == 0 {
		color.Yellow("⚠️  No supported languages detected in current directory")
		return nil
	}

	if verbose {
		fmt.Printf("Detected languages: %s\n", strings.Join(languages, ", "))
	}

	hasErrors := false

	// Run multi-language tools first
	if secrets {
		if err := runGitleaksScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Secrets scan failed: %v", err)
		}
	}

	// Run language-specific tools
	for _, lang := range languages {
		switch lang {
		case "go":
			if err := runGoScans(verbose, security, static, vulns); err != nil {
				hasErrors = true
			}
		case "javascript", "typescript":
			if err := runJavaScriptScans(verbose, security, static, vulns); err != nil {
				hasErrors = true
			}
		case "python":
			if err := runPythonScans(verbose, security, static, vulns); err != nil {
				hasErrors = true
			}
		case "java":
			if err := runJavaScans(verbose, security, static, vulns); err != nil {
				hasErrors = true
			}
		case "rust":
			if err := runRustScans(verbose, security, static, vulns); err != nil {
				hasErrors = true
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("one or more scans detected issues")
	}

	color.Green("✅ All enabled scans completed successfully")
	return nil
}

func detectLanguages() []string {
	languages := []string{}

	// Check for Go
	if fileExists("go.mod") || hasFilesWithExtension(".go") {
		languages = append(languages, "go")
	}

	// Check for JavaScript/TypeScript
	if fileExists("package.json") || hasFilesWithExtension(".js", ".ts", ".jsx", ".tsx") {
		if hasFilesWithExtension(".ts", ".tsx") {
			languages = append(languages, "typescript")
		} else {
			languages = append(languages, "javascript")
		}
	}

	// Check for Python
	if fileExists("requirements.txt") || fileExists("pyproject.toml") || fileExists("setup.py") || hasFilesWithExtension(".py") {
		languages = append(languages, "python")
	}

	// Check for Java
	if fileExists("pom.xml") || fileExists("build.gradle") || hasFilesWithExtension(".java") {
		languages = append(languages, "java")
	}

	// Check for Rust
	if fileExists("Cargo.toml") || hasFilesWithExtension(".rs") {
		languages = append(languages, "rust")
	}

	return languages
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func hasFilesWithExtension(extensions ...string) bool {
	found := false
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip common directories that shouldn't be scanned
			if strings.HasPrefix(info.Name(), ".") ||
				info.Name() == "node_modules" ||
				info.Name() == "vendor" ||
				info.Name() == "target" {
				return filepath.SkipDir
			}
			return nil
		}

		for _, ext := range extensions {
			if strings.HasSuffix(path, ext) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

func runGitleaksScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Scanning for committed secrets with gitleaks..."
	s.Start()

	if !isGitleaksInstalled() {
		s.Stop()
		color.Yellow("⚠️  gitleaks is not installed. Install it with:")
		fmt.Println("   brew install gitleaks")
		fmt.Println("   or visit: https://github.com/gitleaks/gitleaks")
		return nil
	}

	args := []string{"detect", "--verbose"}
	if !verbose {
		args = []string{"detect"}
	}

	cmd := exec.Command("gitleaks", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 Secrets detected!")
				fmt.Println(string(output))
				return fmt.Errorf("gitleaks found potential secrets in the repository")
			}
		}
		return fmt.Errorf("failed to run gitleaks: %v", err)
	}

	color.Green("✅ No secrets detected by gitleaks")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func isGitleaksInstalled() bool {
	_, err := exec.LookPath("gitleaks")
	return err == nil
}

func runGosecScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running Go security analysis with gosec..."
	s.Start()

	if !isToolInstalled("gosec") {
		s.Stop()
		color.Yellow("⚠️  gosec is not installed. Install it with:")
		fmt.Println("   go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest")
		return nil
	}

	args := []string{"./..."}
	if verbose {
		args = append([]string{"-verbose"}, args...)
	}

	cmd := exec.Command("gosec", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 Security issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("gosec found security issues")
			}
		}
		return fmt.Errorf("failed to run gosec: %v", err)
	}

	color.Green("✅ No security issues detected by gosec")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func runStaticcheckScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running static analysis with staticcheck..."
	s.Start()

	if !isToolInstalled("staticcheck") {
		s.Stop()
		color.Yellow("⚠️  staticcheck is not installed. Install it with:")
		fmt.Println("   go install honnef.co/go/tools/cmd/staticcheck@latest")
		return nil
	}

	args := []string{"./..."}

	cmd := exec.Command("staticcheck", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 Static analysis issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("staticcheck found issues")
			}
		}
		return fmt.Errorf("failed to run staticcheck: %v", err)
	}

	if len(output) > 0 {
		color.Red("🚨 Static analysis issues detected!")
		fmt.Println(string(output))
		return fmt.Errorf("staticcheck found issues")
	}

	color.Green("✅ No static analysis issues detected by staticcheck")
	if verbose {
		fmt.Println("staticcheck completed successfully")
	}

	return nil
}

func runGoVetScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running go vet analysis..."
	s.Start()

	args := []string{"vet", "./..."}

	cmd := exec.Command("go", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		color.Red("🚨 Go vet issues detected!")
		fmt.Println(string(output))
		return fmt.Errorf("go vet found issues")
	}

	color.Green("✅ No issues detected by go vet")
	if verbose && len(output) > 0 {
		fmt.Println(string(output))
	}

	return nil
}

func runGovulncheckScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Checking for known vulnerabilities with govulncheck..."
	s.Start()

	if !isToolInstalled("govulncheck") {
		s.Stop()
		color.Yellow("⚠️  govulncheck is not installed. Install it with:")
		fmt.Println("   go install golang.org/x/vuln/cmd/govulncheck@latest")
		return nil
	}

	args := []string{"./..."}

	cmd := exec.Command("govulncheck", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 3 {
				color.Red("🚨 Known vulnerabilities detected!")
				fmt.Println(string(output))
				return fmt.Errorf("govulncheck found vulnerabilities")
			}
		}
		return fmt.Errorf("failed to run govulncheck: %v", err)
	}

	color.Green("✅ No known vulnerabilities detected by govulncheck")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func isToolInstalled(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

// Language-specific scan functions

func runGoScans(verbose, security, static, vulns bool) error {
	hasErrors := false

	if static {
		if err := runGoVetScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Go vet failed: %v", err)
		}

		if err := runStaticcheckScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Static analysis failed: %v", err)
		}
	}

	if security {
		if err := runGosecScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Security scan failed: %v", err)
		}
	}

	if vulns {
		if err := runGovulncheckScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Vulnerability scan failed: %v", err)
		}
	}

	if hasErrors {
		return fmt.Errorf("Go scans detected issues")
	}
	return nil
}

func runJavaScriptScans(verbose, security, static, vulns bool) error {
	hasErrors := false

	if static {
		if err := runESLintScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ ESLint failed: %v", err)
		}
	}

	if vulns {
		if err := runNpmAuditScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ NPM audit failed: %v", err)
		}
	}

	if hasErrors {
		return fmt.Errorf("JavaScript/TypeScript scans detected issues")
	}
	return nil
}

func runPythonScans(verbose, security, static, vulns bool) error {
	hasErrors := false

	if security {
		if err := runBanditScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Bandit security scan failed: %v", err)
		}
	}

	if static {
		if err := runPylintScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Pylint failed: %v", err)
		}
	}

	if vulns {
		if err := runSafetyScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Safety vulnerability scan failed: %v", err)
		}
	}

	if hasErrors {
		return fmt.Errorf("Python scans detected issues")
	}
	return nil
}

func runJavaScans(verbose, security, static, vulns bool) error {
	hasErrors := false

	if static {
		if err := runSpotBugsScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ SpotBugs failed: %v", err)
		}

		if err := runPMDScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ PMD failed: %v", err)
		}
	}

	if hasErrors {
		return fmt.Errorf("Java scans detected issues")
	}
	return nil
}

func runRustScans(verbose, security, static, vulns bool) error {
	hasErrors := false

	if static {
		if err := runCargoClippyScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Cargo clippy failed: %v", err)
		}
	}

	if vulns {
		if err := runCargoAuditScan(verbose); err != nil {
			hasErrors = true
			color.Red("❌ Cargo audit failed: %v", err)
		}
	}

	if hasErrors {
		return fmt.Errorf("Rust scans detected issues")
	}
	return nil
}

// JavaScript/TypeScript tool implementations

func runESLintScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running ESLint analysis..."
	s.Start()

	if !isToolInstalled("eslint") {
		s.Stop()
		color.Yellow("⚠️  eslint is not installed. Install it with:")
		fmt.Println("   npm install -g eslint")
		return nil
	}

	args := []string{".", "--ext", ".js,.jsx,.ts,.tsx"}
	if !verbose {
		args = append(args, "--quiet")
	}

	cmd := exec.Command("eslint", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 ESLint issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("eslint found issues")
			}
		}
		return fmt.Errorf("failed to run eslint: %v", err)
	}

	color.Green("✅ No ESLint issues detected")
	if verbose && len(output) > 0 {
		fmt.Println(string(output))
	}

	return nil
}

func runNpmAuditScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running NPM audit..."
	s.Start()

	if !isToolInstalled("npm") {
		s.Stop()
		color.Yellow("⚠️  npm is not installed")
		return nil
	}

	args := []string{"audit"}
	if !verbose {
		args = append(args, "--audit-level", "moderate")
	}

	cmd := exec.Command("npm", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 NPM vulnerabilities detected!")
				fmt.Println(string(output))
				return fmt.Errorf("npm audit found vulnerabilities")
			}
		}
		return fmt.Errorf("failed to run npm audit: %v", err)
	}

	color.Green("✅ No NPM vulnerabilities detected")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

// Python tool implementations

func runBanditScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running Bandit security analysis..."
	s.Start()

	if !isToolInstalled("bandit") {
		s.Stop()
		color.Yellow("⚠️  bandit is not installed. Install it with:")
		fmt.Println("   pip install bandit")
		return nil
	}

	args := []string{"-r", "."}
	if !verbose {
		args = append(args, "-q")
	}

	cmd := exec.Command("bandit", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 Bandit security issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("bandit found security issues")
			}
		}
		return fmt.Errorf("failed to run bandit: %v", err)
	}

	color.Green("✅ No security issues detected by Bandit")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func runPylintScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running Pylint analysis..."
	s.Start()

	if !isToolInstalled("pylint") {
		s.Stop()
		color.Yellow("⚠️  pylint is not installed. Install it with:")
		fmt.Println("   pip install pylint")
		return nil
	}

	args := []string{"**/*.py"}
	if !verbose {
		args = append(args, "--errors-only")
	}

	cmd := exec.Command("pylint", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Pylint exit codes: 0=no issues, 1-32=various issues found
			if exitError.ExitCode() <= 32 {
				color.Red("🚨 Pylint issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("pylint found issues")
			}
		}
		return fmt.Errorf("failed to run pylint: %v", err)
	}

	color.Green("✅ No Pylint issues detected")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func runSafetyScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running Safety vulnerability check..."
	s.Start()

	if !isToolInstalled("safety") {
		s.Stop()
		color.Yellow("⚠️  safety is not installed. Install it with:")
		fmt.Println("   pip install safety")
		return nil
	}

	args := []string{"check"}
	if !verbose {
		args = append(args, "--short-report")
	}

	cmd := exec.Command("safety", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 64 {
				color.Red("🚨 Safety vulnerabilities detected!")
				fmt.Println(string(output))
				return fmt.Errorf("safety found vulnerabilities")
			}
		}
		return fmt.Errorf("failed to run safety: %v", err)
	}

	color.Green("✅ No vulnerabilities detected by Safety")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

// Java tool implementations

func runSpotBugsScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running SpotBugs analysis..."
	s.Start()

	if !isToolInstalled("spotbugs") {
		s.Stop()
		color.Yellow("⚠️  spotbugs is not installed. Install it from:")
		fmt.Println("   https://spotbugs.github.io/")
		return nil
	}

	args := []string{"-textui", "."}

	cmd := exec.Command("spotbugs", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 SpotBugs issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("spotbugs found issues")
			}
		}
		return fmt.Errorf("failed to run spotbugs: %v", err)
	}

	color.Green("✅ No issues detected by SpotBugs")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func runPMDScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running PMD analysis..."
	s.Start()

	if !isToolInstalled("pmd") {
		s.Stop()
		color.Yellow("⚠️  PMD is not installed. Install it from:")
		fmt.Println("   https://pmd.github.io/")
		return nil
	}

	args := []string{"check", "-d", ".", "-R", "rulesets/java/quickstart.xml", "-f", "text"}

	cmd := exec.Command("pmd", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 4 {
				color.Red("🚨 PMD issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("pmd found issues")
			}
		}
		return fmt.Errorf("failed to run pmd: %v", err)
	}

	color.Green("✅ No issues detected by PMD")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

// Rust tool implementations

func runCargoClippyScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running Cargo clippy..."
	s.Start()

	if !isToolInstalled("cargo") {
		s.Stop()
		color.Yellow("⚠️  cargo is not installed")
		return nil
	}

	args := []string{"clippy", "--", "-D", "warnings"}

	cmd := exec.Command("cargo", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 101 {
				color.Red("🚨 Clippy issues detected!")
				fmt.Println(string(output))
				return fmt.Errorf("cargo clippy found issues")
			}
		}
		return fmt.Errorf("failed to run cargo clippy: %v", err)
	}

	color.Green("✅ No issues detected by Cargo clippy")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}

func runCargoAuditScan(verbose bool) error {
	s := spinner.New(spinner.CharSets[14], 100)
	s.Suffix = " Running Cargo audit..."
	s.Start()

	if !isToolInstalled("cargo-audit") {
		s.Stop()
		color.Yellow("⚠️  cargo-audit is not installed. Install it with:")
		fmt.Println("   cargo install cargo-audit")
		return nil
	}

	args := []string{"audit"}

	cmd := exec.Command("cargo", args...)
	output, err := cmd.CombinedOutput()
	s.Stop()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				color.Red("🚨 Cargo audit vulnerabilities detected!")
				fmt.Println(string(output))
				return fmt.Errorf("cargo audit found vulnerabilities")
			}
		}
		return fmt.Errorf("failed to run cargo audit: %v", err)
	}

	color.Green("✅ No vulnerabilities detected by Cargo audit")
	if verbose {
		fmt.Println(string(output))
	}

	return nil
}
