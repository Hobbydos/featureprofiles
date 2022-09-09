// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package per_component_reboot_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
)

const (
	controlcardType   = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	linecardType      = telemetry.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
	activeController  = telemetry.PlatformTypes_ComponentRedundantRole_PRIMARY
	standbyController = telemetry.PlatformTypes_ComponentRedundantRole_SECONDARY
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Issue gnoi.system Reboot to chassis with
//     - Delay: Not set.
//     - message: Not set.
//     - method: Only the COLD method is required to be supported by all targets.
//     - subcomponents: Standby RP/supervisor or linecard name.
//  2) Set the subcomponent to a standby RP (supervisor).
//     - Verify that the standby RP has rebooted and the uptime has been reset.
//  3) Set the subcomponent to a a field-removable linecard in the system.
//     - Verify that the line card has rebooted and the uptime has been reset.
//
// Topology:
//   DUT
//
// Test notes:
//  - Reboot causes the target to reboot, possibly at some point in the future.
//    If the method of reboot is not supported then the Reboot RPC will fail.
//    If the reboot is immediate the command will block until the subcomponents
//    have restarted.
//    If a reboot on the active control processor is pending the service must
//    reject all other reboot requests.
//    If a reboot request for active control processor is initiated with other
//    pending reboot requests it must be rejected.
//  - Only standby RP/supervisor reboot is tested
//    - Active RP/RP/supervisor reboot might not be supported for some platforms.
//    - Chassis reboot or RP switchover should be performed instead of active
//      RP/RP/supervisor reboot in real world.
//
//  - TODO: Check the uptime has been reset after the reboot.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestStandbyControllerCardReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	supervisors := findComponentsByType(t, dut, controlcardType)
	t.Logf("Found supervisor list: %v", supervisors)
	// Only perform the standby RP rebooting for the chassis with dual RPs/Supervisors.
	if len(supervisors) != 2 {
		t.Skipf("Dual RP/SUP is required on %v: got %v, want 2", dut.Model(), len(supervisors))
	}

	rpStandby, rpActive := findStandbyRP(t, dut, supervisors)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandby, rpActive)

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			{
				Elem: []*tpb.PathElem{{Name: rpStandby}},
			},
		},
	}

	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	startReboot := time.Now()
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform component reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	watch := dut.Telemetry().Component(rpStandby).RedundantRole().Watch(
		t, 10*time.Minute, func(val *telemetry.QualifiedE_PlatformTypes_ComponentRedundantRole) bool {
			return val.IsPresent()
		})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("DUT did not reach target state within %s minutes: got %v", 10*time.Minute.String(), val.String())
	}
	t.Logf("Standby controller boot time: %.2f seconds", time.Since(startReboot).Seconds())

	// TODO: Check the standby RP uptime has been reset.
}

func TestLinecardReboot(t *testing.T) {
	const linecardBoottime = 10 * time.Minute
	dut := ondatra.DUT(t, "dut")

	lcs := findComponentsByType(t, dut, linecardType)
	t.Logf("Found linecard list: %v", lcs)
	if got := len(lcs); got == 0 {
		t.Errorf("Get number of Linecards on %v: got %v, want > 0", dut.Model(), got)
	}

	t.Logf("Find a removable line card to reboot.")
	var removableLinecard string
	for _, lc := range lcs {
		t.Logf("Check if %s is removable", lc)
		if got := dut.Telemetry().Component(lc).Removable().Lookup(t).IsPresent(); !got {
			t.Logf("Detected non-removable line card: %v", lc)
			continue
		}
		if got := dut.Telemetry().Component(lc).Removable().Get(t); got {
			t.Logf("Found removable line card: %v", lc)
			removableLinecard = lc
		}
	}
	if removableLinecard == "" {
		t.Fatalf("Component(lc).Removable().Get(t): got none, want non-empty")
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	rebootSubComponentRequest := &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Subcomponents: []*tpb.Path{
			{
				Elem: []*tpb.PathElem{{Name: removableLinecard}},
			},
		},
	}

	intfsOperStatusUPBeforeReboot := fetchOperStatusUPIntfs(t, dut)
	t.Logf("OperStatusUP interfaces before reboot: %v", intfsOperStatusUPBeforeReboot)
	t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
	if err != nil {
		t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

	rebootDeadline := time.Now().Add(linecardBoottime)
	for retry := true; retry; {
		t.Log("Wating for 10 seconds before checking.")
		time.Sleep(10 * time.Second)
		if time.Now().After(rebootDeadline) {
			retry = false
			break
		}
		resp, err := gnoiClient.System().RebootStatus(context.Background(), &spb.RebootStatusRequest{})
		switch {
		case status.Code(err) == codes.Unimplemented:
			t.Fatalf("Unimplemented RebootStatus() is not fully compliant with the Reboot spec.")
		case err == nil:
			retry = resp.GetActive()
		default:
			// any other error just sleep.
		}
	}

	t.Logf("Validate removable linecard %v status", removableLinecard)
	dut.Telemetry().Component(removableLinecard).Removable().Await(t, linecardBoottime, true)

	t.Logf("Validate interface OperStatus.")
	batch := dut.Telemetry().NewBatch()
	for _, port := range intfsOperStatusUPBeforeReboot {
		dut.Telemetry().Interface(port).OperStatus().Batch(t, batch)
	}
	watch := batch.Watch(t, 5*time.Minute, func(val *telemetry.QualifiedDevice) bool {
		for _, port := range intfsOperStatusUPBeforeReboot {
			if val.Val(t).GetInterface(port).GetOperStatus() != telemetry.Interface_OperStatus_UP {
				return false
			}
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("DUT did not reach target state: got %v", val)
	}

	intfsOperStatusUPAfterReboot := fetchOperStatusUPIntfs(t, dut)
	t.Logf("OperStatusUP interfaces after reboot: %v", intfsOperStatusUPAfterReboot)
	if diff := cmp.Diff(intfsOperStatusUPAfterReboot, intfsOperStatusUPBeforeReboot); diff != "" {
		t.Errorf("OperStatusUP interfaces differed (-want +got):\n%v", diff)
	}

	// TODO: Check the line card uptime has been reset.
}

func findComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := dut.Telemetry().ComponentAny().Name().Get(t)
	var s []string
	for _, c := range components {
		lookupType := dut.Telemetry().Component(c).Type().Lookup(t)
		if !lookupType.IsPresent() {
			t.Logf("Component %s type is not found", c)
		} else {
			componentType := lookupType.Val(t)
			t.Logf("Component %s has type: %v", c, componentType)

			switch v := componentType.(type) {
			case telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
				if v == cType {
					s = append(s, c)
				}
			default:
				t.Fatalf("Expected component type to be a hardware component, got (%T, %v)", componentType, componentType)
			}
		}
	}
	return s
}

func findStandbyRP(t *testing.T, dut *ondatra.DUTDevice, supervisors []string) (string, string) {
	var activeRP, standbyRP string
	for _, supervisor := range supervisors {
		watch := dut.Telemetry().Component(supervisor).RedundantRole().Watch(
			t, 5*time.Minute, func(val *telemetry.QualifiedE_PlatformTypes_ComponentRedundantRole) bool {
				return val.IsPresent()
			})
		if val, ok := watch.Await(t); !ok {
			t.Fatalf("DUT did not reach target state: got %v", val.String())
		}
		role := dut.Telemetry().Component(supervisor).RedundantRole().Get(t)
		t.Logf("Component(supervisor).RedundantRole().Get(t): %v, Role: %v", supervisor, role)
		if role == standbyController {
			standbyRP = supervisor
		} else if role == activeController {
			activeRP = supervisor
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", supervisor, role)
		}
	}
	if standbyRP == "" || activeRP == "" {
		t.Fatalf("Expected non-empty activeRP and standbyRP, got activeRP: %v, standbyRP: %v", activeRP, standbyRP)
	}
	t.Logf("Detected activeRP: %v, standbyRP: %v", activeRP, standbyRP)

	return standbyRP, activeRP
}

func fetchOperStatusUPIntfs(t *testing.T, dut *ondatra.DUTDevice) []string {
	intfsOperStatusUP := []string{}
	intfs := dut.Telemetry().InterfaceAny().Name().Get(t)
	for _, intf := range intfs {
		operStatus := dut.Telemetry().Interface(intf).OperStatus().Lookup(t)
		if operStatus.IsPresent() && operStatus.Val(t) == telemetry.Interface_OperStatus_UP {
			intfsOperStatusUP = append(intfsOperStatusUP, intf)
		}
	}
	sort.Strings(intfsOperStatusUP)
	return intfsOperStatusUP
}
