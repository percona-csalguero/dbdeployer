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

package sandbox

import (
	"fmt"
	"path"
	"regexp"
	"time"

	"github.com/datacharmer/dbdeployer/common"
	"github.com/datacharmer/dbdeployer/concurrent"
	"github.com/datacharmer/dbdeployer/defaults"
)

const (
	GroupReplOptions string = `
binlog_checksum=NONE
log_slave_updates=ON
plugin-load=group_replication.so
group_replication=FORCE_PLUS_PERMANENT
group_replication_start_on_boot=OFF
group_replication_bootstrap_group=OFF
transaction_write_set_extraction=XXHASH64
report-host=127.0.0.1
loose-group_replication_group_name="aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
`
	GroupReplSinglePrimary string = `
loose-group-replication-single-primary-mode=on
`
	GroupReplMultiPrimary string = `
loose-group-replication-single-primary-mode=off
`
)

func getBaseMysqlxPort(basePort int, sdef SandboxDef, nodes int) (error, int) {
	baseMysqlxPort := basePort + defaults.Defaults().MysqlXPortDelta
	// 8.0.11
	if common.GreaterOrEqualVersion(sdef.Version, defaults.MinimumMysqlxDefaultVersion) {
		// FindFreePort returns the first free port, but base_port will be used
		// with a counter. Thus the availability will be checked using
		// "base_port + 1"
		firstGroupPort := common.FindFreePort(baseMysqlxPort+1, sdef.InstalledPorts, nodes)
		baseMysqlxPort = firstGroupPort - 1
		for N := 1; N <= nodes; N++ {
			checkPort := baseMysqlxPort + N
			err := CheckPort("get_base_mysqlx_port", sdef.SandboxDir, sdef.InstalledPorts, checkPort)
			if err != nil {
				return err, 0
			}
		}
	}
	return nil, baseMysqlxPort
}

func CreateGroupReplication(sandboxDef SandboxDef, origin string, nodes int, masterIp string) error {
	var execLists []concurrent.ExecutionList
	var err error

	var logger *defaults.Logger
	if sandboxDef.Logger != nil {
		logger = sandboxDef.Logger
	} else {
		var fileName string
		var err error
		err, fileName, logger = defaults.NewLogger(common.LogDirName(), "group-replication")
		if err != nil {
			return err
		}
		sandboxDef.LogFileName = common.ReplaceLiteralHome(fileName)
	}

	vList := common.VersionToList(sandboxDef.Version)
	rev := vList[2]
	basePort := sandboxDef.Port + defaults.Defaults().GroupReplicationBasePort + (rev * 100)
	if sandboxDef.SinglePrimary {
		basePort = sandboxDef.Port + defaults.Defaults().GroupReplicationSpBasePort + (rev * 100)
	}
	if sandboxDef.BasePort > 0 {
		basePort = sandboxDef.BasePort
	}

	baseServerId := 0
	if nodes < 3 {
		return fmt.Errorf("Can't run group replication with less than 3 nodes")
	}
	if common.DirExists(sandboxDef.SandboxDir) {
		err, sandboxDef = CheckDirectory(sandboxDef)
		if err != nil {
			return err
		}
	}
	// FindFreePort returns the first free port, but base_port will be used
	// with a counter. Thus the availability will be checked using
	// "base_port + 1"
	firstGroupPort := common.FindFreePort(basePort+1, sandboxDef.InstalledPorts, nodes)
	basePort = firstGroupPort - 1
	baseGroupPort := basePort + defaults.Defaults().GroupPortDelta
	firstGroupPort = common.FindFreePort(baseGroupPort+1, sandboxDef.InstalledPorts, nodes)
	baseGroupPort = firstGroupPort - 1
	for checkPort := basePort + 1; checkPort < basePort+nodes+1; checkPort++ {
		err = CheckPort("CreateGroupReplication", sandboxDef.SandboxDir, sandboxDef.InstalledPorts, checkPort)
		if err != nil {
			return err
		}
	}
	for checkPort := baseGroupPort + 1; checkPort < baseGroupPort+nodes+1; checkPort++ {
		err = CheckPort("CreateGroupReplication-group", sandboxDef.SandboxDir, sandboxDef.InstalledPorts, checkPort)
		if err != nil {
			return err
		}
	}
	err, baseMysqlxPort := getBaseMysqlxPort(basePort, sandboxDef, nodes)
	if err != nil {
		return err
	}
	common.Mkdir(sandboxDef.SandboxDir)
	common.AddToCleanupStack(common.Rmdir, "Rmdir", sandboxDef.SandboxDir)
	logger.Printf("Creating directory %s\n", sandboxDef.SandboxDir)
	timestamp := time.Now()
	slaveLabel := defaults.Defaults().SlavePrefix
	slaveAbbr := defaults.Defaults().SlaveAbbr
	masterAbbr := defaults.Defaults().MasterAbbr
	masterLabel := defaults.Defaults().MasterName
	masterList := makeNodesList(nodes)
	slaveList := masterList
	if sandboxDef.SinglePrimary {
		masterList = "1"
		slaveList = ""
		for N := 2; N <= nodes; N++ {
			if slaveList != "" {
				slaveList += " "
			}
			slaveList += fmt.Sprintf("%d", N)
		}
		err, mlist := nodesListToIntSlice(masterList, nodes)
		if err != nil {
			return err
		}
		err, slist := nodesListToIntSlice(slaveList, nodes)
		if err != nil {
			return err
		}
		err = checkNodeLists(nodes, mlist, slist)
		if err != nil {
			return err
		}
	}
	changeMasterExtra := ""
	nodeLabel := defaults.Defaults().NodePrefix
	//if common.GreaterOrEqualVersion(sdef.Version, []int{8,0,4}) {
	//	if !sdef.NativeAuthPlugin {
	//		change_master_extra = ", GET_MASTER_PUBLIC_KEY=1"
	//	}
	//}
	var data = common.StringMap{
		"Copyright":         Copyright,
		"AppVersion":        common.VersionDef,
		"DateTime":          timestamp.Format(time.UnixDate),
		"SandboxDir":        sandboxDef.SandboxDir,
		"MasterIp":          masterIp,
		"MasterList":        masterList,
		"NodeLabel":         nodeLabel,
		"SlaveList":         slaveList,
		"RplUser":           sandboxDef.RplUser,
		"RplPassword":       sandboxDef.RplPassword,
		"SlaveLabel":        slaveLabel,
		"SlaveAbbr":         slaveAbbr,
		"ChangeMasterExtra": changeMasterExtra,
		"MasterLabel":       masterLabel,
		"MasterAbbr":        masterAbbr,
		"Nodes":             []common.StringMap{},
	}
	connectionString := ""
	for i := 0; i < nodes; i++ {
		groupPort := baseGroupPort + i + 1
		if connectionString != "" {
			connectionString += ","
		}
		connectionString += fmt.Sprintf("127.0.0.1:%d", groupPort)
	}
	logger.Printf("Creating connection string %s\n", connectionString)

	sbType := "group-multi-primary"
	singleMultiPrimary := GroupReplMultiPrimary
	if sandboxDef.SinglePrimary {
		sbType = "group-single-primary"
		singleMultiPrimary = GroupReplSinglePrimary
	}
	logger.Printf("Defining group type %s\n", sbType)

	sbDesc := common.SandboxDescription{
		Basedir: sandboxDef.Basedir,
		SBType:  sbType,
		Version: sandboxDef.Version,
		Port:    []int{},
		Nodes:   nodes,
		NodeNum: 0,
		LogFile: sandboxDef.LogFileName,
	}

	sbItem := defaults.SandboxItem{
		Origin:      sbDesc.Basedir,
		SBType:      sbDesc.SBType,
		Version:     sandboxDef.Version,
		Port:        []int{},
		Nodes:       []string{},
		Destination: sandboxDef.SandboxDir,
	}

	if sandboxDef.LogFileName != "" {
		sbItem.LogDirectory = common.DirName(sandboxDef.LogFileName)
	}

	for i := 1; i <= nodes; i++ {
		groupPort := baseGroupPort + i
		data["Nodes"] = append(data["Nodes"].([]common.StringMap), common.StringMap{
			"Copyright":         Copyright,
			"AppVersion":        common.VersionDef,
			"DateTime":          timestamp.Format(time.UnixDate),
			"Node":              i,
			"MasterIp":          masterIp,
			"NodeLabel":         nodeLabel,
			"SlaveLabel":        slaveLabel,
			"SlaveAbbr":         slaveAbbr,
			"ChangeMasterExtra": changeMasterExtra,
			"MasterLabel":       masterLabel,
			"MasterAbbr":        masterAbbr,
			"SandboxDir":        sandboxDef.SandboxDir,
			"RplUser":           sandboxDef.RplUser,
			"RplPassword":       sandboxDef.RplPassword})

		sandboxDef.DirName = fmt.Sprintf("%s%d", nodeLabel, i)
		sandboxDef.Port = basePort + i
		sandboxDef.MorePorts = []int{groupPort}
		sandboxDef.ServerId = (baseServerId + i) * 100
		sbItem.Nodes = append(sbItem.Nodes, sandboxDef.DirName)
		sbItem.Port = append(sbItem.Port, sandboxDef.Port)
		sbDesc.Port = append(sbDesc.Port, sandboxDef.Port)
		sbItem.Port = append(sbItem.Port, sandboxDef.Port+defaults.Defaults().GroupPortDelta)
		sbDesc.Port = append(sbDesc.Port, sandboxDef.Port+defaults.Defaults().GroupPortDelta)

		if !sandboxDef.RunConcurrently {
			installationMessage := "Installing and starting %s %d\n"
			if sandboxDef.SkipStart {
				installationMessage = "Installing %s %d\n"
			}
			fmt.Printf(installationMessage, nodeLabel, i)
			logger.Printf(installationMessage, nodeLabel, i)
		}
		sandboxDef.ReplOptions = SingleTemplates["replication_options"].Contents + fmt.Sprintf("\n%s\n%s\n", GroupReplOptions, singleMultiPrimary)
		reMasterIp := regexp.MustCompile(`127\.0\.0\.1`)
		sandboxDef.ReplOptions = reMasterIp.ReplaceAllString(sandboxDef.ReplOptions, masterIp)
		sandboxDef.ReplOptions += fmt.Sprintf("\n%s\n", SingleTemplates["gtid_options_57"].Contents)
		sandboxDef.ReplOptions += fmt.Sprintf("\n%s\n", SingleTemplates["repl_crash_safe_options"].Contents)
		sandboxDef.ReplOptions += fmt.Sprintf("\nloose-group-replication-local-address=%s:%d\n", masterIp, groupPort)
		sandboxDef.ReplOptions += fmt.Sprintf("\nloose-group-replication-group-seeds=%s\n", connectionString)
		// 8.0.11
		if common.GreaterOrEqualVersion(sandboxDef.Version, defaults.MinimumMysqlxDefaultVersion) {
			sandboxDef.MysqlXPort = baseMysqlxPort + i
			if !sandboxDef.DisableMysqlX {
				sbDesc.Port = append(sbDesc.Port, baseMysqlxPort+i)
				sbItem.Port = append(sbItem.Port, baseMysqlxPort+i)
				logger.Printf("adding port %d to node %d\n", baseMysqlxPort+i, i)
			}
		}
		sandboxDef.Multi = true
		sandboxDef.LoadGrants = true
		sandboxDef.Prompt = fmt.Sprintf("%s%d", nodeLabel, i)
		sandboxDef.SBType = "group-node"
		sandboxDef.NodeNum = i
		// fmt.Printf("%#v\n",sdef)
		logger.Printf("Create single sandbox for node %d\n", i)
		execList, err := CreateSingleConcurrentSandbox(sandboxDef)
		if err != nil {
			common.Exit(1, err.Error())
		}
		for _, list := range execList {
			execLists = append(execLists, list)
		}
		var dataNode = common.StringMap{
			"Copyright":         Copyright,
			"AppVersion":        common.VersionDef,
			"DateTime":          timestamp.Format(time.UnixDate),
			"Node":              i,
			"NodeLabel":         nodeLabel,
			"MasterLabel":       masterLabel,
			"MasterAbbr":        masterAbbr,
			"ChangeMasterExtra": changeMasterExtra,
			"SlaveLabel":        slaveLabel,
			"SlaveAbbr":         slaveAbbr,
			"SandboxDir":        sandboxDef.SandboxDir,
		}
		logger.Printf("Create node script for node %d\n", i)
		err = writeScript(logger, MultipleTemplates, fmt.Sprintf("n%d", i), "node_template", sandboxDef.SandboxDir, dataNode, true)
		if err != nil {
			return err
		}
	}
	logger.Printf("Writing sandbox description in %s\n", sandboxDef.SandboxDir)
	common.WriteSandboxDescription(sandboxDef.SandboxDir, sbDesc)
	defaults.UpdateCatalog(sandboxDef.SandboxDir, sbItem)

	logger.Printf("Writing group replication scripts\n")
	sbMultiple := ScriptBatch{
		tc:         MultipleTemplates,
		logger:     logger,
		data:       data,
		sandboxDir: sandboxDef.SandboxDir,
		scripts: []ScriptDef{
			{defaults.ScriptStartAll, "start_multi_template", true},
			{defaults.ScriptRestartAll, "restart_multi_template", true},
			{defaults.ScriptStatusAll, "status_multi_template", true},
			{defaults.ScriptTestSbAll, "test_sb_multi_template", true},
			{defaults.ScriptStopAll, "stop_multi_template", true},
			{defaults.ScriptClearAll, "clear_multi_template", true},
			{defaults.ScriptSendKillAll, "send_kill_multi_template", true},
			{defaults.ScriptUseAll, "use_multi_template", true},
		},
	}
	sbRepl := ScriptBatch{
		tc:         ReplicationTemplates,
		logger:     logger,
		data:       data,
		sandboxDir: sandboxDef.SandboxDir,
		scripts: []ScriptDef{
			{defaults.ScriptUseAllSlaves, "multi_source_use_slaves_template", true},
			{defaults.ScriptUseAllMasters, "multi_source_use_masters_template", true},
			{defaults.ScriptTestReplication, "multi_source_test_template", true},
		},
	}
	sbGroup := ScriptBatch{
		tc:         GroupTemplates,
		logger:     logger,
		data:       data,
		sandboxDir: sandboxDef.SandboxDir,
		scripts: []ScriptDef{
			{defaults.ScriptInitializeNodes, "init_nodes_template", true},
			{defaults.ScriptCheckNodes, "check_nodes_template", true},
		},
	}

	for _, sb := range []ScriptBatch{sbMultiple, sbRepl, sbGroup} {
		err := writeScripts(sb)
		if err != nil {
			return err
		}
	}

	logger.Printf("Running parallel tasks\n")
	concurrent.RunParallelTasksByPriority(execLists)
	if !sandboxDef.SkipStart {
		fmt.Println(path.Join(common.ReplaceLiteralHome(sandboxDef.SandboxDir), defaults.ScriptInitializeNodes))
		logger.Printf("Running group replication initialization script\n")
		err, _ := common.RunCmd(path.Join(sandboxDef.SandboxDir, defaults.ScriptInitializeNodes))
		if err != nil {
			return fmt.Errorf("error initializing group replication: %s", err)
		}
	}
	fmt.Printf("Replication directory installed in %s\n", common.ReplaceLiteralHome(sandboxDef.SandboxDir))
	fmt.Printf("run 'dbdeployer usage multiple' for basic instructions'\n")
	return nil
}
