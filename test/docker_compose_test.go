package test

import (
	"testing"
	"fmt"
	"path/filepath"
	"github.com/gruntwork-io/terratest/test-structure"
	terralog "github.com/gruntwork-io/terratest/log"
	"log"
	"github.com/gruntwork-io/terratest/shell"
	"strconv"
	"github.com/gruntwork-io/terratest/util"
)

func TestUnitCouchbaseInDocker(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		testName string
		examplesFolderName string
		osName string
		clusterSize int
		couchbaseWebConsolePort int
		syncGatewayWebConsolePort int
	} {
		{"TestUnitCouchbaseSingleClusterUbuntuInDocker","couchbase-single-cluster", "ubuntu", 2, 8091, 4984},
		{"TestUnitCouchbaseMultiClusterAmazonLinuxInDocker", "couchbase-multi-cluster", "amazon-linux", 3,7091, 3984},
	}

	for _, testCase := range testCases {
		testCase := testCase // capture range variable; otherwise, only the very last test case will run!

		t.Run(testCase.testName, func(t *testing.T) {
			// Disable parallelism, as running more than one Docker test at a time in CircleCi, for some reason,
			// causes them to fail.
			//t.Parallel()
			testCouchbaseInDocker(t, testCase.testName, testCase.examplesFolderName, testCase.osName, testCase.clusterSize, testCase.couchbaseWebConsolePort, testCase.syncGatewayWebConsolePort)
		})
	}
}


func testCouchbaseInDocker(t *testing.T, testName string, examplesFolderName string, osName string, clusterSize int, couchbaseWebConsolePort int, syncGatewayWebConsolePort int) {
	logger := terralog.NewLogger(testName)

	tmpExamplesDir := test_structure.CopyTerraformFolderToTemp(t, "../", "examples", testName, logger)
	couchbaseAmiDir := filepath.Join(tmpExamplesDir, "couchbase-ami")
	couchbaseSingleClusterDockerDir := filepath.Join(tmpExamplesDir, examplesFolderName, "local-test")
	uniqueId := util.UniqueId()

	test_structure.RunTestStage("setup_image", logger, func() {
		buildCouchbaseWithPacker(logger, fmt.Sprintf("%s-docker", osName), "couchbase","us-east-1", couchbaseAmiDir)
	})

	test_structure.RunTestStage("setup_docker", logger, func() {
		startCouchbaseWithDockerCompose(t, couchbaseSingleClusterDockerDir, testName, osName, couchbaseWebConsolePort, syncGatewayWebConsolePort, uniqueId, logger)
	})

	defer test_structure.RunTestStage("teardown", logger, func() {
		getDockerComposeLogs(t, couchbaseSingleClusterDockerDir, testName, osName, couchbaseWebConsolePort, syncGatewayWebConsolePort, uniqueId, logger)
		stopCouchbaseWithDockerCompose(t, couchbaseSingleClusterDockerDir, testName, osName, couchbaseWebConsolePort, syncGatewayWebConsolePort, uniqueId, logger)
	})

	test_structure.RunTestStage("validation", logger, func() {
		consoleUrl := fmt.Sprintf("http://localhost:%d", couchbaseWebConsolePort)
		checkCouchbaseConsoleIsRunning(t, consoleUrl, logger)

		dataNodesUrl := fmt.Sprintf("http://%s:%s@localhost:%d", usernameForTest, passwordForTest, couchbaseWebConsolePort)
		checkCouchbaseClusterIsInitialized(t, dataNodesUrl, clusterSize, logger)
		checkCouchbaseDataNodesWorking(t, dataNodesUrl, logger)

		syncGatewayUrl := fmt.Sprintf("http://localhost:%d/mock-couchbase-asg", syncGatewayWebConsolePort)
		checkSyncGatewayWorking(t, syncGatewayUrl, logger)
	})
}

func startCouchbaseWithDockerCompose(t *testing.T, exampleDir string, testName string, osName string, webConsolePort int, syncGatewayPort int, uniqueId string, logger *log.Logger) {
	runDockerCompose(t, exampleDir, testName, osName, webConsolePort, syncGatewayPort, uniqueId, logger, "up", "-d")
}

func getDockerComposeLogs(t *testing.T, exampleDir string, testName string, osName string, webConsolePort int, syncGatewayPort int, uniqueId string, logger *log.Logger) {
	logger.Printf("Fetching docker-compose logs:")
	runDockerCompose(t, exampleDir, testName, osName, webConsolePort, syncGatewayPort, uniqueId, logger, "logs")
}

func stopCouchbaseWithDockerCompose(t *testing.T, exampleDir string, testName string, osName string, webConsolePort int, syncGatewayPort int, uniqueId string, logger *log.Logger) {
	runDockerCompose(t, exampleDir, testName, osName, webConsolePort, syncGatewayPort, uniqueId, logger, "down")
	runDockerCompose(t, exampleDir, testName, osName, webConsolePort, syncGatewayPort, uniqueId, logger, "rm", "-f")
}

func runDockerCompose(t *testing.T, exampleDir string, testName string, osName string, webConsolePort int, syncGatewayPort int, uniqueId string, logger *log.Logger, args ... string) {
	cmd := shell.Command{
		Command:    "docker-compose",
		// We append --project-name to ensure containers from multiple different tests using Docker Compose don't end
		// up in the same project and end up conflicting with each other.
		Args:       append([]string{"--project-name", testName}, args...),
		WorkingDir: exampleDir,
		Env: map[string]string{
			"OS_NAME": osName,
			"WEB_CONSOLE_PORT": strconv.Itoa(webConsolePort),
			"SYNC_GATEWAY_PORT": strconv.Itoa(syncGatewayPort),
			"CONTAINER_BASE_NAME": fmt.Sprintf("couchbase-%s", uniqueId),
		},
	}

	if err := shell.RunCommand(cmd, logger); err != nil {
		t.Fatalf("Failed to run docker-compose %v in %s: %v", args, exampleDir, err)
	}
}