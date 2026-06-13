package main

import (
	"os"

	"github.com/Ozhiaki/inferctl/pkg/inferctl"
)

func deterministicOutputMode() bool {
	return os.Getenv("INFERCTL_TEST_DETERMINISTIC") == "1"
}

func normalizeBackendInfoForOutput(info inferctl.BackendInfo) inferctl.BackendInfo {
	if deterministicOutputMode() && info.LatencyMS != nil {
		zero := 0
		info.LatencyMS = &zero
	}
	return info
}

func normalizeBackendStatusForOutput(status inferctl.BackendStatus) inferctl.BackendStatus {
	status.BackendInfo = normalizeBackendInfoForOutput(status.BackendInfo)
	return status
}
