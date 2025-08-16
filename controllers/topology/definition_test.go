package topology_test

import (
	"encoding/json"
	"fmt"
	"testing"

	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconfig "github.com/srl-labs/clabernetes/config"
	clabernetescontrollerstopology "github.com/srl-labs/clabernetes/controllers/topology"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	clabernetestesthelper "github.com/srl-labs/clabernetes/testhelper"
	clabernetesutil "github.com/srl-labs/clabernetes/util"
	clabernetesutilcontainerlab "github.com/srl-labs/clabernetes/util/containerlab"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const definitionProcessTestName = "definition"

func TestDefinitionProcess(t *testing.T) {
	cases := []struct {
		name                 string
		inTopology           *clabernetesapisv1alpha1.Topology
		reconcileData        *clabernetescontrollerstopology.ReconcileData
		removeTopologyPrefix bool
	}{
		{
			name: "containerlab-host-and-links",
			inTopology: &clabernetesapisv1alpha1.Topology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "process-containerlab-definition-host-and-links-test",
					Namespace: "clabernetes",
				},
				Spec: clabernetesapisv1alpha1.TopologySpec{
					Definition: clabernetesapisv1alpha1.Definition{
						Containerlab: `---
    name: test
    topology:
      nodes:
        srl1:
          kind: srl
          image: ghcr.io/nokia/srlinux
        srl2:
          kind: srl
          image: ghcr.io/nokia/srlinux
          healthcheck:
            test:
              - CMD-SHELL
              - cat /etc/os-release
            start-period: 3
            retries: 1
            interval: 5
            timeout: 2
      links:
        - endpoints: ["srl1:e1-1", "srl2:e1-1"]
        - endpoints: ["srl1:e3-3", "host:eth3-3"]
`,
					},
				},
			},
			reconcileData: &clabernetescontrollerstopology.ReconcileData{
				Kind:           "containerlab",
				ResolvedHashes: clabernetesapisv1alpha1.ReconcileHashes{},
				ResolvedConfigs: map[string]*clabernetesutilcontainerlab.Config{
					"srl1": {},
					"srl2": {
						Name: "clabernetes-srl2",
						Topology: &clabernetesutilcontainerlab.Topology{
							Defaults: &clabernetesutilcontainerlab.NodeDefinition{
								Ports: []string{},
							},
							Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
								"srl2": {
									Ports: []string{},
									Kind:  "srl",
									Image: "ghcr.io/nokia/srlinux",
									Healthcheck: &clabernetesutilcontainerlab.HealthcheckConfig{
										Test:        []string{"CMD-SHELL", "cat /etc/os-release"},
										StartPeriod: 3,
										Retries:     1,
										Interval:    5,
										Timeout:     2,
									},
								},
							},
						},
					},
				},
				ResolvedTunnels: map[string][]*clabernetesapisv1alpha1.PointToPointTunnel{
					"srl1": {},
					"srl2": {},
				},
			},
			removeTopologyPrefix: false,
		},
		{
			name: "containerlab-host-link",
			inTopology: &clabernetesapisv1alpha1.Topology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "process-containerlab-definition-host-link-test",
					Namespace: "clabernetes",
				},
				Spec: clabernetesapisv1alpha1.TopologySpec{
					Definition: clabernetesapisv1alpha1.Definition{
						Containerlab: `---
    name: test
    topology:
      nodes:
        srl1:
          kind: srl
          image: ghcr.io/nokia/srlinux
      links:
        - endpoints: ["srl1:e1-1", "host:srl1-e1-1"]
`,
					},
				},
			},
			reconcileData: &clabernetescontrollerstopology.ReconcileData{
				Kind:           "containerlab",
				ResolvedHashes: clabernetesapisv1alpha1.ReconcileHashes{},
				ResolvedConfigs: map[string]*clabernetesutilcontainerlab.Config{
					"srl1": {},
				},
				ResolvedTunnels: map[string][]*clabernetesapisv1alpha1.PointToPointTunnel{},
			},
			removeTopologyPrefix: false,
		},
		{
			name: "containerlab-simple",
			inTopology: &clabernetesapisv1alpha1.Topology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "process-containerlab-definition-test",
					Namespace: "clabernetes",
				},
				Spec: clabernetesapisv1alpha1.TopologySpec{
					Definition: clabernetesapisv1alpha1.Definition{
						Containerlab: `---
    name: test
    topology:
      nodes:
        srl1:
          kind: srl
          image: ghcr.io/nokia/srlinux
        srl2:
          kind: srl
          image: ghcr.io/nokia/srlinux
      links:
        - endpoints: ["srl1:e1-1", "srl2:e1-1"]
`,
					},
				},
			},
			reconcileData: &clabernetescontrollerstopology.ReconcileData{
				Kind:           "containerlab",
				ResolvedHashes: clabernetesapisv1alpha1.ReconcileHashes{},
				ResolvedConfigs: map[string]*clabernetesutilcontainerlab.Config{
					"srl1": {},
					"srl2": {},
				},
				ResolvedTunnels: map[string][]*clabernetesapisv1alpha1.PointToPointTunnel{
					"srl1": {},
					"srl2": {},
				},
			},
			removeTopologyPrefix: false,
		},
		{
			name: "containerlab-simple-remove-prefix",
			inTopology: &clabernetesapisv1alpha1.Topology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "process-containerlab-definition-test",
					Namespace: "clabernetes",
				},
				Spec: clabernetesapisv1alpha1.TopologySpec{
					Definition: clabernetesapisv1alpha1.Definition{
						Containerlab: `---
    name: test
    topology:
      nodes:
        srl1:
          kind: srl
          image: ghcr.io/nokia/srlinux
        srl2:
          kind: srl
          image: ghcr.io/nokia/srlinux
      links:
        - endpoints: ["srl1:e1-1", "srl2:e1-1"]
`,
					},
				},
				Status: clabernetesapisv1alpha1.TopologyStatus{
					RemoveTopologyPrefix: clabernetesutil.ToPointer(true),
				},
			},
			reconcileData: &clabernetescontrollerstopology.ReconcileData{
				Kind:           "containerlab",
				ResolvedHashes: clabernetesapisv1alpha1.ReconcileHashes{},
				ResolvedConfigs: map[string]*clabernetesutilcontainerlab.Config{
					"srl1": {},
					"srl2": {},
				},
				ResolvedTunnels: map[string][]*clabernetesapisv1alpha1.PointToPointTunnel{
					"srl1": {},
					"srl2": {},
				},
			},
			removeTopologyPrefix: true,
		},
	}

	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				t.Logf("%s: starting", testCase.name)

				processor, err := clabernetescontrollerstopology.NewDefinitionProcessor(
					&claberneteslogging.FakeInstance{},
					testCase.inTopology,
					testCase.reconcileData,
					clabernetesconfig.GetFakeManager,
				)
				if err != nil {
					t.Fatal(err)
				}

				err = processor.Process()
				if err != nil {
					t.Fatal(err)
				}

				got := testCase.reconcileData

				if *clabernetestesthelper.Update {
					clabernetestesthelper.WriteTestFixtureJSON(
						t,
						fmt.Sprintf(
							"golden/%s/%s.json",
							definitionProcessTestName,
							testCase.name,
						),
						got,
					)
				}

				var want *clabernetescontrollerstopology.ReconcileData

				err = json.Unmarshal(
					clabernetestesthelper.ReadTestFixtureFile(
						t,
						fmt.Sprintf(
							"golden/%s/%s.json",
							definitionProcessTestName,
							testCase.name,
						),
					),
					&want,
				)
				if err != nil {
					t.Fatal(err)
				}

				clabernetestesthelper.MarshaledEqual(t, got, want)
			},
		)
	}
}
