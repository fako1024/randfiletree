package randfiletree

import "fmt"

func ExampleBuildBuiltInScenario() {
	const scenarioName = ScenarioNameHardlinkHeavy
	const scenarioSeed int64 = 20260522

	spec, err := BuildBuiltInScenario(scenarioName, scenarioSeed)
	if err != nil {
		fmt.Println("build error:", err)
		return
	}

	fmt.Println(spec.Descriptor.Name)
	fmt.Println(spec.Seed)
	// Output:
	// hardlink-heavy
	// 20260522
}
