// Copyright 2023 Google LLC
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

package mixed_sp_wrr_traffic_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type trafficData struct {
	trafficRate           float64
	expectedThroughputPct float32
	frameSize             uint32
	dscp                  uint8
	queue                 string
	inputIntf             *ondatra.Interface
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - https://github.com/openconfig/featureprofiles/blob/main/feature/experimental/qos/ate_tests/mixed_sp_wrr_traffic_test/README.md
//
// Topology:
//       ATE port 1
//        |
//       DUT--------ATE port 3
//        |
//       ATE port 2
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestQoSCounters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	// Configure DUT interfaces and QoS.
	ConfigureDUTIntf(t, dut)
	ConfigureQoS(t, dut)

	// Configure ATE interfaces.
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")
	top := ate.Topology().New()
	intf1 := top.AddInterface("intf1").WithPort(ap1)
	intf1.IPv4().
		WithAddress("198.51.100.1/31").
		WithDefaultGateway("198.51.100.0")
	intf2 := top.AddInterface("intf2").WithPort(ap2)
	intf2.IPv4().
		WithAddress("198.51.100.3/31").
		WithDefaultGateway("198.51.100.2")
	intf3 := top.AddInterface("intf3").WithPort(ap3)
	intf3.IPv4().
		WithAddress("198.51.100.5/31").
		WithDefaultGateway("198.51.100.4")
	top.Push(t).StartProtocols(t)

	var tolerance float32 = 2.0

	queueMap := map[ondatra.Vendor]map[string]string{
		ondatra.JUNIPER: {
			"NC1": "3",
			"AF4": "2",
			"AF3": "5",
			"AF2": "1",
			"AF1": "4",
			"BE1": "0",
			"BE0": "6",
		},
		ondatra.ARISTA: {
			"NC1": dp3.Name() + "-7",
			"AF4": dp3.Name() + "-4",
			"AF3": dp3.Name() + "-3",
			"AF2": dp3.Name() + "-2",
			"AF1": dp3.Name() + "-0",
			"BE1": dp3.Name() + "-1",
			"BE0": dp3.Name() + "-1",
		},
		ondatra.CISCO: {
			"NC1": "7",
			"AF4": "4",
			"AF3": "3",
			"AF2": "2",
			"AF1": "0",
			"BE1": "1",
			"BE0": "1",
		},
		ondatra.NOKIA: {
			"NC1": "7",
			"AF4": "4",
			"AF3": "3",
			"AF2": "2",
			"AF1": "0",
			"BE1": "1",
			"BE0": "1",
		},
	}

	NonoversubscribedTrafficFlows := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             700,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             400,
			trafficRate:           18,
			expectedThroughputPct: 100.0,
			dscp:                  32,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             1300,
			trafficRate:           16,
			expectedThroughputPct: 100.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             1200,
			trafficRate:           8,
			expectedThroughputPct: 100.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             1000,
			trafficRate:           4,
			expectedThroughputPct: 100.0,
			dscp:                  8,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             1111,
			trafficRate:           2,
			expectedThroughputPct: 100.0,
			dscp:                  0,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf1,
		},
		"intf1-be0": {
			frameSize:             1110,
			trafficRate:           0.5,
			dscp:                  4,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             700,
			trafficRate:           0.9,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             400,
			trafficRate:           20,
			dscp:                  32,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             1300,
			trafficRate:           16,
			expectedThroughputPct: 100.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             1200,
			trafficRate:           8,
			expectedThroughputPct: 100.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             1000,
			trafficRate:           4,
			expectedThroughputPct: 100.0,
			dscp:                  8,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             1111,
			trafficRate:           2,
			dscp:                  0,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf2,
		},
		"intf2-be0": {
			frameSize:             1112,
			trafficRate:           0.5,
			expectedThroughputPct: 100.0,
			dscp:                  5,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf2,
		},
	}

	oversubscribedTrafficFlows1 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             700,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             400,
			trafficRate:           50,
			expectedThroughputPct: 100.0,
			dscp:                  32,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             1300,
			trafficRate:           20,
			expectedThroughputPct: 0.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             1200,
			trafficRate:           14,
			expectedThroughputPct: 0.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             1000,
			trafficRate:           12,
			expectedThroughputPct: 0.0,
			dscp:                  8,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf1,
		},
		"intf1-be1": {
			frameSize:             1111,
			trafficRate:           1,
			expectedThroughputPct: 0.0,
			dscp:                  0,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf1,
		},
		"intf1-be0": {
			frameSize:             1110,
			trafficRate:           1,
			dscp:                  4,
			expectedThroughputPct: 0.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             700,
			trafficRate:           0.9,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             400,
			trafficRate:           49,
			dscp:                  32,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             1300,
			trafficRate:           14,
			expectedThroughputPct: 0.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             1200,
			trafficRate:           24,
			expectedThroughputPct: 0.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             1000,
			trafficRate:           4,
			expectedThroughputPct: 0.0,
			dscp:                  8,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             1111,
			trafficRate:           7,
			dscp:                  0,
			expectedThroughputPct: 0.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf2,
		},
		"intf2-be0": {
			frameSize:             1112,
			trafficRate:           1,
			expectedThroughputPct: 0.0,
			dscp:                  5,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf2,
		},
	}

	oversubscribedTrafficFlows2 := map[string]*trafficData{
		"intf1-nc1": {
			frameSize:             700,
			trafficRate:           0.1,
			expectedThroughputPct: 100.0,
			dscp:                  56,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf1,
		},
		"intf1-af4": {
			frameSize:             400,
			trafficRate:           18,
			expectedThroughputPct: 100.0,
			dscp:                  32,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf1,
		},
		"intf1-af3": {
			frameSize:             1300,
			trafficRate:           40,
			expectedThroughputPct: 50.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf1,
		},
		"intf1-af2": {
			frameSize:             1200,
			trafficRate:           8,
			expectedThroughputPct: 50.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf1,
		},
		"intf1-af1": {
			frameSize:             1000,
			trafficRate:           12,
			expectedThroughputPct: 50.0,
			dscp:                  8, queue: queueMap[dut.Vendor()]["AF1"],
			inputIntf: intf1,
		},
		"intf1-be1": {
			frameSize:             1111,
			trafficRate:           1,
			expectedThroughputPct: 50.0,
			dscp:                  0,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf1,
		},
		"intf1-be0": {
			frameSize:             1110,
			trafficRate:           1,
			dscp:                  4,
			expectedThroughputPct: 50.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf1,
		},
		"intf2-nc1": {
			frameSize:             700,
			trafficRate:           0.9,
			dscp:                  56,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["NC1"],
			inputIntf:             intf2,
		},
		"intf2-af4": {
			frameSize:             400,
			trafficRate:           20,
			dscp:                  32,
			expectedThroughputPct: 100.0,
			queue:                 queueMap[dut.Vendor()]["AF4"],
			inputIntf:             intf2,
		},
		"intf2-af3": {
			frameSize:             1300,
			trafficRate:           24,
			expectedThroughputPct: 50.0,
			dscp:                  24,
			queue:                 queueMap[dut.Vendor()]["AF3"],
			inputIntf:             intf2,
		},
		"intf2-af2": {
			frameSize:             1200,
			trafficRate:           24,
			expectedThroughputPct: 50.0,
			dscp:                  16,
			queue:                 queueMap[dut.Vendor()]["AF2"],
			inputIntf:             intf2,
		},
		"intf2-af1": {
			frameSize:             1000,
			trafficRate:           4,
			expectedThroughputPct: 50.0,
			dscp:                  8,
			queue:                 queueMap[dut.Vendor()]["AF1"],
			inputIntf:             intf2,
		},
		"intf2-be1": {
			frameSize:             1111,
			trafficRate:           7,
			dscp:                  0,
			expectedThroughputPct: 50.0,
			queue:                 queueMap[dut.Vendor()]["BE1"],
			inputIntf:             intf2,
		},
		"intf2-be0": {
			frameSize:             1112,
			trafficRate:           1,
			expectedThroughputPct: 50.0,
			dscp:                  5,
			queue:                 queueMap[dut.Vendor()]["BE0"],
			inputIntf:             intf2,
		},
	}

	cases := []struct {
		desc         string
		trafficFlows map[string]*trafficData
	}{{
		desc:         "Non-oversubscription traffic",
		trafficFlows: NonoversubscribedTrafficFlows,
	}, {
		desc:         "Oversubscription traffic with all BE0-AF3 dropped",
		trafficFlows: oversubscribedTrafficFlows1,
	}, {
		desc:         "Oversubscription traffic with half BE0-AF3 dropped",
		trafficFlows: oversubscribedTrafficFlows2,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			trafficFlows := tc.trafficFlows

			var flows []*ondatra.Flow
			for trafficID, data := range trafficFlows {
				t.Logf("Configuring flow %s", trafficID)
				flow := ate.Traffic().NewFlow(trafficID).
					WithSrcEndpoints(data.inputIntf).
					WithDstEndpoints(intf3).
					WithHeaders(ondatra.NewEthernetHeader(), ondatra.NewIPv4Header().WithDSCP(data.dscp)).
					WithFrameRatePct(data.trafficRate).
					WithFrameSize(data.frameSize)
				flows = append(flows, flow)
			}

			ateOutPkts := make(map[string]uint64)
			dutQosPktsBeforeTraffic := make(map[string]uint64)
			dutQosPktsAfterTraffic := make(map[string]uint64)
			dutQosDroppedPktsBeforeTraffic := make(map[string]uint64)
			dutQosDroppedPktsAfterTraffic := make(map[string]uint64)

			// Get QoS egress packet counters before the traffic.
			for _, data := range trafficFlows {
				dutQosPktsBeforeTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				dutQosDroppedPktsBeforeTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State())
			}

			t.Logf("Running traffic 1 on DUT interfaces: %s => %s ", dp1.Name(), dp3.Name())
			t.Logf("Running traffic 2 on DUT interfaces: %s => %s ", dp2.Name(), dp3.Name())
			t.Logf("Sending traffic flows: \n%v\n\n", trafficFlows)
			ate.Traffic().Start(t, flows...)
			time.Sleep(10 * time.Second)
			ate.Traffic().Stop(t)
			time.Sleep(30 * time.Second)

			for trafficID, data := range trafficFlows {
				ateOutPkts[data.queue] = gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).Counters().OutPkts().State())
				dutQosPktsAfterTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).TransmitPkts().State())
				dutQosDroppedPktsAfterTraffic[data.queue] = gnmi.Get(t, dut, gnmi.OC().Qos().Interface(dp3.Name()).Output().Queue(data.queue).DroppedPkts().State())
				t.Logf("ateOutPkts: %v, txPkts %v, Queue: %v", ateOutPkts[data.queue], dutQosPktsAfterTraffic[data.queue], data.queue)

				lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(trafficID).LossPct().State())
				t.Logf("Get flow %q: lossPct: %.2f%% or rxPct: %.2f%%, want: %.2f%%\n\n", data.queue, lossPct, 100.0-lossPct, data.expectedThroughputPct)
				if got, want := 100.0-lossPct, data.expectedThroughputPct; got < want-tolerance || got > want+tolerance {
					t.Errorf("Get(throughput for queue %q): got %.2f%%, want within [%.2f%%, %.2f%%]", data.queue, got, want-tolerance, want+tolerance)
				}
			}

			// Check QoS egress packet counters are updated correctly.
			t.Logf("QoS egress packet counters before traffic: %v", dutQosPktsBeforeTraffic)
			t.Logf("QoS egress packet counters after traffic: %v", dutQosPktsAfterTraffic)
			t.Logf("QoS egress dropped packet counters before traffic: %v", dutQosDroppedPktsBeforeTraffic)
			t.Logf("QoS egress dropped packet counters after traffic: %v", dutQosDroppedPktsAfterTraffic)
			t.Logf("QoS packet counters from ATE: %v", ateOutPkts)
			for _, data := range trafficFlows {
				qosCounterDiff := dutQosPktsAfterTraffic[data.queue] - dutQosPktsBeforeTraffic[data.queue]
				if qosCounterDiff < ateOutPkts[data.queue] {
					t.Errorf("Get telemetry packet update for queue %q: got %v, want >= %v", data.queue, qosCounterDiff, ateOutPkts[data.queue])
				}
			}
		})
	}
}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	dutIntfs := []struct {
		desc      string
		intfName  string
		ipAddr    string
		prefixLen uint8
	}{{
		desc:      "Input interface port1",
		intfName:  dp1.Name(),
		ipAddr:    "198.51.100.0",
		prefixLen: 31,
	}, {
		desc:      "Input interface port2",
		intfName:  dp2.Name(),
		ipAddr:    "198.51.100.2",
		prefixLen: 31,
	}, {
		desc:      "Output interface port3",
		intfName:  dp3.Name(),
		ipAddr:    "198.51.100.4",
		prefixLen: 31,
	}}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		i.GetOrCreateEthernet()
		s := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
			s.Enabled = ygot.Bool(true)
		}
		a := s.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.prefixLen)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}

func ConfigureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	t.Logf("Create qos Classifiers config")
	classifiers := []struct {
		desc         string
		name         string
		classType    oc.E_Qos_Classifier_Type
		termID       string
		targetGrpoup string
		dscpSet      []uint8
	}{{
		desc:         "classifier_ipv4_be1",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "0",
		targetGrpoup: "target-group-BE1",
		dscpSet:      []uint8{0, 1, 2, 3},
	}, {
		desc:         "classifier_ipv4_be0",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "1",
		targetGrpoup: "target-group-BE0",
		dscpSet:      []uint8{4, 5, 6, 7},
	}, {
		desc:         "classifier_ipv4_af1",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "2",
		targetGrpoup: "target-group-AF1",
		dscpSet:      []uint8{8, 9, 10, 11},
	}, {
		desc:         "classifier_ipv4_af2",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "3",
		targetGrpoup: "target-group-AF2",
		dscpSet:      []uint8{16, 17, 18, 19},
	}, {
		desc:         "classifier_ipv4_af3",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "4",
		targetGrpoup: "target-group-AF3",
		dscpSet:      []uint8{24, 25, 26, 27},
	}, {
		desc:         "classifier_ipv4_af4",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "5",
		targetGrpoup: "target-group-AF4",
		dscpSet:      []uint8{32, 33, 34, 35},
	}, {
		desc:         "classifier_ipv4_nc1",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "6",
		targetGrpoup: "target-group-NC1",
		dscpSet:      []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
	}, {
		desc:         "classifier_ipv6_be1",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "0",
		targetGrpoup: "target-group-BE1",
		dscpSet:      []uint8{0, 1, 2, 3},
	}, {
		desc:         "classifier_ipv6_be0",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "1",
		targetGrpoup: "target-group-BE0",
		dscpSet:      []uint8{4, 5, 6, 7},
	}, {
		desc:         "classifier_ipv6_af1",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "2",
		targetGrpoup: "target-group-AF1",
		dscpSet:      []uint8{8, 9, 10, 11},
	}, {
		desc:         "classifier_ipv6_af2",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "3",
		targetGrpoup: "target-group-AF2",
		dscpSet:      []uint8{16, 17, 18, 19},
	}, {
		desc:         "classifier_ipv6_af3",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "4",
		targetGrpoup: "target-group-AF3",
		dscpSet:      []uint8{24, 25, 26, 27},
	}, {
		desc:         "classifier_ipv6_af4",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "5",
		targetGrpoup: "target-group-AF4",
		dscpSet:      []uint8{32, 33, 34, 35},
	}, {
		desc:         "classifier_ipv6_nc1",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "6",
		targetGrpoup: "target-group-NC1",
		dscpSet:      []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
	}}

	t.Logf("qos Classifiers config: %v", classifiers)
	for _, tc := range classifiers {
		t.Run(tc.desc, func(t *testing.T) {
			classifier := q.GetOrCreateClassifier(tc.name)
			classifier.SetName(tc.name)
			classifier.SetType(tc.classType)
			term, err := classifier.NewTerm(tc.termID)
			if err != nil {
				t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
			}

			term.SetId(tc.termID)
			action := term.GetOrCreateActions()
			action.SetTargetGroup(tc.targetGrpoup)
			condition := term.GetOrCreateConditions()
			if tc.name == "dscp_based_classifier_ipv4" {
				condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
			} else if tc.name == "dscp_based_classifier_ipv6" {
				condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
			}
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
	}

	t.Logf("Create qos input classifier config")
	classifierIntfs := []struct {
		desc                string
		intf                string
		inputClassifierType oc.E_Input_Classifier_Type
		classifier          string
	}{{
		desc:                "Input Classifier Type IPV4",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp1.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}, {
		desc:                "Input Classifier Type IPV4",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
	}, {
		desc:                "Input Classifier Type IPV6",
		intf:                dp2.Name(),
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
	}}

	t.Logf("qos input classifier config: %v", classifierIntfs)
	for _, tc := range classifierIntfs {
		t.Run(tc.desc, func(t *testing.T) {
			i := q.GetOrCreateInterface(tc.intf)
			i.SetInterfaceId(tc.intf)
			c := i.GetOrCreateInput().GetOrCreateClassifier(tc.inputClassifierType)
			c.SetType(tc.inputClassifierType)
			c.SetName(tc.classifier)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
	}

	t.Logf("Create qos forwarding groups config")
	forwardingGroups := []struct {
		desc         string
		queueName    string
		targetGrpoup string
	}{{
		desc:         "forwarding-group-BE1",
		queueName:    "BE1",
		targetGrpoup: "target-group-BE1",
	}, {
		desc:         "forwarding-group-BE0",
		queueName:    "BE0",
		targetGrpoup: "target-group-BE0",
	}, {
		desc:         "forwarding-group-AF1",
		queueName:    "AF1",
		targetGrpoup: "target-group-AF1",
	}, {
		desc:         "forwarding-group-AF2",
		queueName:    "AF2",
		targetGrpoup: "target-group-AF2",
	}, {
		desc:         "forwarding-group-AF3",
		queueName:    "AF3",
		targetGrpoup: "target-group-AF3",
	}, {
		desc:         "forwarding-group-AF4",
		queueName:    "AF4",
		targetGrpoup: "target-group-AF4",
	}, {
		desc:         "forwarding-group-NC1",
		queueName:    "NC1",
		targetGrpoup: "target-group-NC1",
	}}

	t.Logf("qos forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		t.Run(tc.desc, func(t *testing.T) {
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGrpoup)
			fwdGroup.SetName(tc.targetGrpoup)
			fwdGroup.SetOutputQueue(tc.queueName)
			queue := q.GetOrCreateQueue(tc.queueName)
			queue.SetName(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
	}

	t.Logf("Create qos scheduler policies config")
	schedulerPolicies := []struct {
		desc         string
		sequence     uint32
		priority     oc.E_Scheduler_Priority
		inputID      string
		inputType    oc.E_Input_InputType
		weight       uint64
		queueName    string
		targetGrpoup string
	}{{
		desc:         "scheduler-policy-BE1",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputID:      "BE1",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(1),
		queueName:    "BE1",
		targetGrpoup: "target-group-BE1",
	}, {
		desc:         "scheduler-policy-BE0",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputID:      "BE0",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(4),
		queueName:    "BE0",
		targetGrpoup: "target-group-BE0",
	}, {
		desc:         "scheduler-policy-AF1",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputID:      "AF1",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(8),
		queueName:    "AF1",
		targetGrpoup: "target-group-AF1",
	}, {
		desc:         "scheduler-policy-AF2",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputID:      "AF2",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(16),
		queueName:    "AF2",
		targetGrpoup: "target-group-AF2",
	}, {
		desc:         "scheduler-policy-AF3",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputID:      "AF3",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(32),
		queueName:    "AF3",
		targetGrpoup: "target-group-AF3",
	}, {
		desc:         "scheduler-policy-AF4",
		sequence:     uint32(0),
		priority:     oc.Scheduler_Priority_STRICT,
		inputID:      "AF4",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(100),
		queueName:    "AF4",
		targetGrpoup: "target-group-AF4",
	}, {
		desc:         "scheduler-policy-NC1",
		sequence:     uint32(0),
		priority:     oc.Scheduler_Priority_STRICT,
		inputID:      "NC1",
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(200),
		queueName:    "NC1",
		targetGrpoup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("qos scheduler policies config: %v", schedulerPolicies)
	for _, tc := range schedulerPolicies {
		t.Run(tc.desc, func(t *testing.T) {
			s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
			s.SetSequence(tc.sequence)
			s.SetPriority(tc.priority)
			input := s.GetOrCreateInput(tc.inputID)
			input.SetId(tc.inputID)
			input.SetInputType(tc.inputType)
			input.SetQueue(tc.queueName)
			input.SetWeight(tc.weight)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
	}

	t.Logf("Create qos output interface config")
	schedulerIntfs := []struct {
		desc      string
		queueName string
		scheduler string
	}{{
		desc:      "output-interface-BE1",
		queueName: "BE1",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-BE0",
		queueName: "BE0",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF1",
		queueName: "AF1",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF2",
		queueName: "AF2",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF3",
		queueName: "AF3",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-AF4",
		queueName: "AF4",
		scheduler: "scheduler",
	}, {
		desc:      "output-interface-NC1",
		queueName: "NC1",
		scheduler: "scheduler",
	}}

	t.Logf("qos output interface config: %v", schedulerIntfs)
	for _, tc := range schedulerIntfs {
		t.Run(tc.desc, func(t *testing.T) {
			i := q.GetOrCreateInterface(dp3.Name())
			i.SetInterfaceId(dp3.Name())
			output := i.GetOrCreateOutput()
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(tc.scheduler)
			queue := output.GetOrCreateQueue(tc.queueName)
			queue.SetName(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
	}
}
