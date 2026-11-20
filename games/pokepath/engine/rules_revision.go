package engine

const NativeRulesRevision = "a74d870e816519f54c32cedea2a21c6ccc84b23a96d752e5da394e029fbe4e44"

func RulesVersion() string {
	return "pokepath-native-1/" + NativeRulesRevision + "/" + CatalogData().Source.SourceSHA256
}
