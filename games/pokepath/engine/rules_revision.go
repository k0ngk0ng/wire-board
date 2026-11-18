package engine

const NativeRulesRevision = "401d71da4e3677ea2f18c69822760e066026d1e96ed303234a43b5e8a7d5a616"

func RulesVersion() string {
	return "pokepath-native-1/" + NativeRulesRevision + "/" + CatalogData().Source.SourceSHA256
}
