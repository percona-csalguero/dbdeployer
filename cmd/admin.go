// DBDeployer - The MySQL Sandbox
// Copyright © 2006-2018 Giuseppe Maxia
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/datacharmer/dbdeployer/common"
	"github.com/datacharmer/dbdeployer/defaults"
	"github.com/datacharmer/dbdeployer/sandbox"
	"github.com/spf13/cobra"
	"os"
	"path"
)

func UnpreserveSandbox(sandboxDir, sandboxName string) {
	fullPath := path.Join(sandboxDir, sandboxName)
	if !common.DirExists(fullPath) {
		common.Exitf(1, defaults.ErrDirectoryNotFound, fullPath)
	}
	preserve := path.Join(fullPath, defaults.ScriptNoClearAll)
	if !common.ExecExists(preserve) {
		preserve = path.Join(fullPath, defaults.ScriptNoClear)
	}
	if !common.ExecExists(preserve) {
		fmt.Printf("Sandbox %s is not locked\n", sandboxName)
		return
	}
	isMultiple := true
	clear := path.Join(fullPath, defaults.ScriptClearAll)
	if !common.ExecExists(clear) {
		clear = path.Join(fullPath, defaults.ScriptClear)
		isMultiple = false
	}
	if !common.ExecExists(clear) {
		common.Exitf(1, defaults.ErrExecutableNotFound, clear)
	}
	noClear := path.Join(fullPath, defaults.ScriptNoClear)
	if isMultiple {
		noClear = path.Join(fullPath, defaults.ScriptNoClearAll)
	}
	err := os.Remove(clear)
	common.ErrCheckExitf(err, 1, defaults.ErrWhileRemoving, clear, err)
	err = os.Rename(noClear, clear)
	common.ErrCheckExitf(err, 1, defaults.ErrWhileRenamingScript, err)
	fmt.Printf("Sandbox %s unlocked\n", sandboxName)
}

func PreserveSandbox(sandboxDir, sandboxName string) {
	fullPath := path.Join(sandboxDir, sandboxName)
	if !common.DirExists(fullPath) {
		common.Exitf(1, defaults.ErrDirectoryNotFound, fullPath)
	}
	preserve := path.Join(fullPath, defaults.ScriptNoClearAll)
	if !common.ExecExists(preserve) {
		preserve = path.Join(fullPath, defaults.ScriptNoClear)
	}
	if common.ExecExists(preserve) {
		fmt.Printf("Sandbox %s is already locked\n", sandboxName)
		return
	}
	isMultiple := true
	clear := path.Join(fullPath, defaults.ScriptClearAll)
	if !common.ExecExists(clear) {
		clear = path.Join(fullPath, defaults.ScriptClear)
		isMultiple = false
	}
	if !common.ExecExists(clear) {
		common.Exitf(1, defaults.ErrExecutableNotFound, clear)
	}
	noClear := path.Join(fullPath, defaults.ScriptNoClear)
	clearCmd := defaults.ScriptClear
	noClearCmd := defaults.ScriptNoClear
	if isMultiple {
		noClear = path.Join(fullPath, defaults.ScriptNoClearAll)
		clearCmd = defaults.ScriptClearAll
		noClearCmd = defaults.ScriptNoClearAll
	}
	err := os.Rename(clear, noClear)
	common.ErrCheckExitf(err, 1, defaults.ErrWhileRenamingScript, err)
	template := sandbox.SingleTemplates["sb_locked_template"].Contents
	var data = common.StringMap{
		"TemplateName": "sb_locked_template",
		"SandboxDir":   sandboxName,
		"AppVersion":   common.VersionDef,
		"Copyright":    sandbox.Copyright,
		"ClearCmd":     clearCmd,
		"NoClearCmd":   noClearCmd,
	}
	template = common.TrimmedLines(template)
	newClearMessage := common.TemplateFill(template, data)
	common.WriteString(newClearMessage, clear)
	os.Chmod(clear, 0744)
	fmt.Printf("Sandbox %s locked\n", sandboxName)
}

func LockSandbox(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		common.Exit(1,
			"'lock' requires the name of a sandbox (or ALL)",
			"Example: dbdeployer admin lock msb_5_7_21")
	}
	candidateSandbox := args[0]
	sandboxDir := GetAbsolutePathFromFlag(cmd, "sandbox-home")
	lockList := []string{candidateSandbox}
	if candidateSandbox == "ALL" || candidateSandbox == "all" {
		lockList = common.SandboxInfoToFileNames(common.GetInstalledSandboxes(sandboxDir))
	}
	if len(lockList) == 0 {
		fmt.Printf("Nothing to lock in %s\n", sandboxDir)
		return
	}
	for _, sb := range lockList {
		PreserveSandbox(sandboxDir, sb)
	}
}

func UnlockSandbox(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		common.Exit(1,
			"'unlock' requires the name of a sandbox (or ALL)",
			"Example: dbdeployer admin unlock msb_5_7_21")
	}
	candidateSandbox := args[0]
	sandboxDir := GetAbsolutePathFromFlag(cmd, "sandbox-home")
	lockList := []string{candidateSandbox}
	if candidateSandbox == "ALL" || candidateSandbox == "all" {
		lockList = common.SandboxInfoToFileNames(common.GetInstalledSandboxes(sandboxDir))
	}
	if len(lockList) == 0 {
		fmt.Printf("Nothing to lock in %s\n", sandboxDir)
		return
	}
	for _, sb := range lockList {
		UnpreserveSandbox(sandboxDir, sb)
	}
}

func UpgradeSandbox(sandboxDir, oldSandbox, newSandbox string) {
	var possibleUpgrades = map[string]string{
		"5.0": "5.1",
		"5.1": "5.5",
		"5.5": "5.6",
		"5.6": "5.7",
		"5.7": "8.0",
		"8.0": "8.0",
	}
	err := os.Chdir(sandboxDir)
	common.ErrCheckExitf(err, 1, "can't change directory to %s", sandboxDir)
	scripts := []string{defaults.ScriptStart, defaults.ScriptStop, defaults.ScriptMy}
	for _, dir := range []string{oldSandbox, newSandbox} {
		if !common.DirExists(dir) {
			common.Exitf(1, defaults.ErrDirectoryNotFoundInUpper, dir, sandboxDir)
		}
		for _, script := range scripts {
			if !common.ExecExists(path.Join(dir, script)) {
				common.Exit(1, fmt.Sprintf(defaults.ErrScriptNotFoundInUpper, script, dir),
					"The upgrade only works between SINGLE deployments")
			}
		}
	}
	newSbdesc := common.ReadSandboxDescription(newSandbox)
	oldSbdesc := common.ReadSandboxDescription(oldSandbox)
	mysqlUpgrade := path.Join(newSbdesc.Basedir, "bin", "mysql_upgrade")
	if !common.ExecExists(mysqlUpgrade) {
		common.WriteString("", path.Join(newSandbox, "no_upgrade"))
		common.Exitf(0, "mysql_upgrade not found in %s. Upgrade is not possible", newSbdesc.Basedir)
	}
	newVersionList := common.VersionToList(newSbdesc.Version)
	newMajor := newVersionList[0]
	newMinor := newVersionList[1]
	newRev := newVersionList[2]
	oldVersionList := common.VersionToList(oldSbdesc.Version)
	oldMajor := oldVersionList[0]
	oldMinor := oldVersionList[1]
	oldRev := oldVersionList[2]
	newUpgradeVersion := fmt.Sprintf("%d.%d", newVersionList[0], newVersionList[1])
	oldUpgradeVersion := fmt.Sprintf("%d.%d", oldVersionList[0], oldVersionList[1])
	if oldMajor == 10 || newMajor == 10 {
		common.Exit(1, "upgrade from and to MariaDB is not supported")
	}
	if common.GreaterOrEqualVersion(oldSbdesc.Version, newVersionList) {
		common.Exitf(1, "version %s must be greater than %s", newUpgradeVersion, oldUpgradeVersion)
	}
	canBeUpgraded := false
	if oldMajor < newMajor {
		canBeUpgraded = true
	} else {
		if oldMajor == newMajor && oldMinor < newMinor {
			canBeUpgraded = true
		} else {
			if oldMajor == newMajor && oldMinor == newMinor && oldRev < newRev {
				canBeUpgraded = true
			}
		}
	}
	if !canBeUpgraded {
		common.Exitf(1, "version '%s' can only be upgraded to '%s' or to the same version with a higher revision", oldUpgradeVersion, possibleUpgrades[oldUpgradeVersion])
	}
	newSandboxOldData := path.Join(newSandbox, defaults.DataDirName+"-"+newSandbox)
	if common.DirExists(newSandboxOldData) {
		common.Exitf(1, "sandbox '%s' is already the upgrade from an older version", newSandbox)
	}
	err, _ = common.RunCmd(path.Join(oldSandbox, defaults.ScriptStop))
	common.ErrCheckExitf(err, 1, defaults.ErrWhileStoppingSandbox, oldSandbox)
	err, _ = common.RunCmd(path.Join(newSandbox, defaults.ScriptStop))
	common.ErrCheckExitf(err, 1, defaults.ErrWhileStoppingSandbox, newSandbox)
	mvArgs := []string{path.Join(newSandbox, defaults.DataDirName), newSandboxOldData}
	err, _ = common.RunCmdWithArgs("mv", mvArgs)
	common.ErrCheckExitf(err, 1, "error while moving data directory in sandbox %s", newSandbox)

	mvArgs = []string{path.Join(oldSandbox, defaults.DataDirName), path.Join(newSandbox, defaults.DataDirName)}
	err, _ = common.RunCmdWithArgs("mv", mvArgs)
	common.ErrCheckExitf(err, 1, "error while moving data directory from sandbox %s to %s", oldSandbox, newSandbox)
	fmt.Printf("Data directory %s/data moved to %s/data \n", oldSandbox, newSandbox)

	err, _ = common.RunCmd(path.Join(newSandbox, defaults.ScriptStart))
	common.ErrCheckExitf(err, 1, defaults.ErrWhileStartingSandbox, newSandbox)
	upgradeArgs := []string{"sql_upgrade"}
	err, _ = common.RunCmdWithArgs(path.Join(newSandbox, defaults.ScriptMy), upgradeArgs)
	common.ErrCheckExitf(err, 1, "error while running mysql_upgrade in %s", newSandbox)
	fmt.Println("")
	fmt.Printf("The data directory from %s/data is preserved in %s\n", newSandbox, newSandboxOldData)
	fmt.Printf("The data directory from %s/data is now used in %s/data\n", oldSandbox, newSandbox)
	fmt.Printf("%s is not operational and can be deleted\n", oldSandbox)
}

func RunUpgradeSandbox(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		common.Exit(1,
			"'upgrade' requires the name of two sandboxes ",
			"Example: dbdeployer admin upgrade msb_5_7_23 msb_8_0_12")
	}
	oldSandbox := args[0]
	newSandbox := args[1]
	sandboxDir := GetAbsolutePathFromFlag(cmd, "sandbox-home")
	UpgradeSandbox(sandboxDir, oldSandbox, newSandbox)
}

var (
	adminCmd = &cobra.Command{
		Use:     "admin",
		Short:   "sandbox management tasks",
		Aliases: []string{"manage"},
		Long:    `Runs commands related to the administration of sandboxes.`,
	}

	adminLockCmd = &cobra.Command{
		Use:     "lock sandbox_name",
		Aliases: []string{"preserve"},
		Short:   "Locks a sandbox, preventing deletion",
		Long: `Prevents deletion for a given sandbox.
Note that the deletion being prevented is only the one occurring through dbdeployer. 
Users can still delete locked sandboxes manually.`,
		Run: LockSandbox,
	}

	adminUnlockCmd = &cobra.Command{
		Use:     "unlock sandbox_name",
		Aliases: []string{"unpreserve"},
		Short:   "Unlocks a sandbox",
		Long:    `Removes lock, allowing deletion of a given sandbox`,
		Run:     UnlockSandbox,
	}
	adminUpgradeCmd = &cobra.Command{
		Use:   "upgrade sandbox_name newer_sandbox",
		Short: "Upgrades a sandbox to a newer version",
		Long: `Upgrades a sandbox to a newer version.
The sandbox with the new version must exist already.
The data directory of the old sandbox will be moved to the new one.`,
		Example: "dbdeployer admin upgrade msb_8_0_11 msb_8_0_12",
		Run:     RunUpgradeSandbox,
	}
)

func init() {
	rootCmd.AddCommand(adminCmd)
	adminCmd.AddCommand(adminLockCmd)
	adminCmd.AddCommand(adminUnlockCmd)
	adminCmd.AddCommand(adminUpgradeCmd)
}
