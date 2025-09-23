package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var addcheatCmd = &cobra.Command{
	Use:   "addcheat [additional instructions]",
	Short: "Add the last command from history to a navi cheat sheet",
	Long:  "Retrieves the last command from shell history and asks Claude Code to add it to an appropriate navi cheat sheet. You can provide additional instructions as arguments.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cheatDir, _ := cmd.Flags().GetString("cheat-directory")

		absCheatDir, err := filepath.Abs(cheatDir)
		if err != nil {
			return fmt.Errorf("invalid cheat directory path: %v", err)
		}

		if _, err := os.Stat(absCheatDir); os.IsNotExist(err) {
			return fmt.Errorf("cheat directory does not exist: %s", absCheatDir)
		}

		lastCommand, err := getLastCommand()
		if err != nil {
			return fmt.Errorf("failed to get last command: %v", err)
		}

		if lastCommand == "" {
			return fmt.Errorf("no command found in history")
		}

		// Combine additional arguments as extra instructions
		additionalInstructions := ""
		if len(args) > 0 {
			additionalInstructions = strings.Join(args, " ")
		}

		if err := sendToClaudeCode(lastCommand, absCheatDir, additionalInstructions); err != nil {
			return fmt.Errorf("failed to send to Claude Code: %v", err)
		}

		fmt.Printf("Sent command to Claude Code for addition to cheat sheets\n")
		return nil
	},
}

func init() {
	defaultDir := filepath.Join(os.Getenv("HOME"), "Thoth", "04-Resources", "navi-cheatsheets")
	addcheatCmd.Flags().StringP("cheat-directory", "d", defaultDir, "Directory containing navi cheat sheets")
}

func getLastCommand() (string, error) {
	histCmd := exec.Command("zsh", "-c", "tail -2 ~/.zsh_history | head -1")
	output, err := histCmd.Output()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))
	if strings.Contains(result, ";") {
		parts := strings.SplitN(result, ";", 2)
		if len(parts) > 1 {
			result = parts[1]
		}
	}

	return result, nil
}

func sendToClaudeCode(command, cheatDir, additionalInstructions string) error {
	prompt := fmt.Sprintf(`Add this command to the appropriate navi cheat sheet in the directory %s:

Command: %s

Instructions:
1. Find the most appropriate existing cheat sheet file (.cheat) in the directory
2. If no appropriate file exists, create a new one with a suitable name
3. Add the command with proper navi syntax, including:
   - A descriptive comment (starting with #)
   - The command itself (starting with $)
   - Any relevant tags or variables if the command has parameters
4. Ensure the formatting follows navi conventions
Here is the exact syntax readme:
# The syntax of a Navi cheatsheet

<!-- TOC -->
* [The syntax of a Navi cheatsheet](#the-syntax-of-a-navi-cheatsheet)
  * [Syntax overview](#syntax-overview)
  * [Variables](#variables)
    * [Advanced variable options](#advanced-variable-options)
    * [Variable dependency](#variable-dependency)
      * [Implicit dependencies](#implicit-dependencies)
      * [Explicit dependencies](#explicit-dependencies)
    * [Variable as multiple arguments](#variable-as-multiple-arguments)
  * [Extending cheats](#extending-cheats)
  * [Multiline commands/snippets](#multiline-commandssnippets)
  * [Aliases](#aliases)
<!-- TOC -->

## Syntax overview

Cheats are described in cheatsheet files.\
A cheatsheet is a file that has a .cheat or .cheat.md extension and looks like this:

% git, code

# Change branch
git checkout <branch>

$ branch: git branch | awk '{print $NF}'

A cheatsheet can have the following elements:

|             Element              | Syntax |                                                                                       Description                                                                                        |
|:--------------------------------:|:------:|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|
|       Tags as cheat titles       |  %   |                                       Lines starting with this character are considered the start of a new cheat command and should contain tags.                                        |
|        Cheat Description         |  #   |                                                Lines starting with this character should be the description of the cheat you're writing.                                                 |
| Cheat Comments (or Metacomments) |  ;   |                                          Lines starting with this character will be ignored by navi but they can be great as editor's comments.                                          |
|      Pre-defined variables       |  $   |   Lines starting with this character should contain commands that generate a list of possible values. <br/> <br/> :information_source: See [#variables](#variables) for more details.    |
|         Extended cheats          |  @   | Lines starting with this character should contain tags associated to other defined cheats. <br/> <br/> :information_source: See [#extending-cheats](#extending-cheats) for more details. |
|       Executable commands        |  N/A   |                                                             All other non-empty lines are considered as executable commands.                                                             |

> [!TIP]
> If you are editing cheatsheets in Visual Studio Code, you could enable syntax highlighting
> by installing this extension: [@yanivmo/navi-cheatsheet-language](https://marketplace.visualstudio.com/items?itemName=yanivmo.navi-cheatsheet-language).

## Variables

Variables are defined with brackets inside executable commands (e.g. <branch>).\
Variable names should only include alphanumeric characters and _.

You can show suggestions by using the Pre-defined variable lines (i.e. lines starting with $).\
Otherwise, the user will be able to type any value for it.

### Advanced variable options

For Pre-Defined variable lines, you can use --- to customize the behavior of fzf
or how the value is going to be used.

Below are examples of such customization:

- We define what column to use, the number of header lines and a delimiter between values.

    # This will pick the 3rd column and use the first line as header
    docker rmi <image_id>
    
    $ image_id: docker images --- --column 3 --header-lines 1 --delimiter '\s\s+'

- We modify the output values of a command

    # Even though "false/true" is displayed, this will print "0/1"
    echo <mapped>

    $ mapped: echo 'false true' | tr ' ' '\n' --- --map "grep -q t && echo 1 || echo 0"


The supported parameters are:

| Parameter               | Description                                                                               |
|:------------------------|:------------------------------------------------------------------------------------------|
| --column <number>     | <number> is the column number to extract from the result.                               |
| --map <bash_code>     | **_[EXPERIMENTAL]_** <bash_code> is a map function to apply to the variable value.      |
| --prevent-extra       | **_[EXPERIMENTAL]_** This parameter will limit the user to select one of the suggestions. |
| --fzf-overrides <arg> | **_[EXPERIMENTAL]_** <arg> is an arbitrary argument to override fzf behaviour.        |
| --expand              | **_[EXPERIMENTAL]_** This parameter will convert each line into a separate argument.      |


In addition, it's possible to forward the following parameters to fzf:

| Parameter forwarded to fzf |
|:-----------------------------|
| --multi                    |
| --header-lines <number>    |
| --delimiter <regex>        |
| --query <text>             |
| --filter <text>            |
| --header <text>            |
| --preview <bash_code>      |
| --preview-window <text>    |

### Variable dependency

Pre-Defined variables can refer other pre-defined variables in two different ways, an implicit and explicit way.

#### Implicit dependencies

An implicit dependency is when you refer another variable with the same syntax used in
executable commands (i.e. <variable>).

Below is an example of using implicit dependencies to construct a path:

sh
# Should print /my/pictures/wallpapers
echo "<wallpaper_folder>"

$ pictures_folder: echo "/my/pictures"
$ wallpaper_folder: echo "<pictures_folder>/wallpapers"


#### Explicit dependencies

An explicit dependency is when you prepend a dollar sign (i.e. $) to the variable name.

Below is an example of using explicit dependencies to give multiple choices:

sh
# If you select "hello" for <x>, the possible values of <y> will be "hello foo" and "hello bar"
echo <x> <y>

# If you want to ignore the contents of <x> and only print <y>
: <x>; echo <y>

$ x: echo "hello hi" | tr ' ' '\n'
$ y: echo "$x foo;$x bar" | tr ';' '\n'


### Variable as multiple arguments

Variables can have multiple arguments,
below is an example of using multiple arguments to cat multiple files at the same time.

sh
# This will result into: cat "file1.json" "file2.json"
cat <jsons>

$ jsons: find . -iname '*.json' -type f -print --- --multi --expand


## Extending cheats

Navi allows you to extend a cheat context with Extended cheats lines (i.e. starting with @).\
If you put the same tags from another cheat, you will be able to share the same context and will
be able to use the same variables, for example.

sh
% dirs, common

$ pictures_folder: echo "/my/pictures"

% wallpapers
@ dirs, common

# Should print /my/pictures/wallpapers
echo "<pictures_folder>/wallpapers"

% screenshots
@ dirs, common

# Should print /my/pictures/screenshots
echo "<pictures_folder>/screenshots"


## Multiline commands/snippets

Commands can be multiline, we call them snippets.

- You can write them as follows:

    sh
    % bash, foo

    # This will output "foo\nyes"
    echo foo
    true \
       && echo yes \
       || echo no
    

- Or, you can place them inside Markdown code blocks, delimited by triple backticks (  ):

    sh
    % git, code
    
    # Change branch
    sh
    git checkout <branch>
    
    
    $ branch: git branch | awk '{print $NF}'
    


## Aliases

**navi** doesn't have support for aliases as first-class citizens at the moment.\
However, it is easy to create aliases using **navi** + a few conventions.

> [!CAUTION]
> The examples below will only work if you use **navi** as a shell scripting tool.
>
> See [/docs/usage/shell-scripting](/docs/usage/shell-scripting/README.md) for more details.

For example, suppose you decide to end some of your commands with :: <some_alias>:

bash
% aliases

# This is one command :: el
echo lorem ipsum

# This is another command :: ef
echo foo bar


You could add something similar to this in your .bashrc-like file:

bash
navialias() {
    navi --query ":: $1" --best-match
}

alias el="navialias el"
alias ef="navialias ef"


If you don't want to use these conventions, you can even add full comments in your aliases:

bash
navibestmatch() {
    navi --query "$1" --best-match
}

alias el="navibestmatch 'This is one command'"
alias ef="navibestmatch 'This is another command'"

Make the addition concise and useful for future reference.

Do everything autonomously without asking for confirmation. Allow all file operations and tool usage.`, cheatDir, command)

	// Add additional instructions if provided
	if additionalInstructions != "" {
		prompt += fmt.Sprintf("\n\nAdditional instructions from user: %s", additionalInstructions)
	}

	claudeCmd := exec.Command("claude", "--dangerously-skip-permissions", prompt, cheatDir)
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr
	claudeCmd.Stdin = os.Stdin

	return claudeCmd.Run()
}
