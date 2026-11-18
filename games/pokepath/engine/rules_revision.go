package engine

const NativeRulesRevision = "d6a01ea5b411fb7c3c2f897823ce338fe74f4850745dcce577aa92a446ba83ac"

func RulesVersion() string {
	return "pokepath-native-1/" + NativeRulesRevision + "/" + CatalogData().Source.SourceSHA256
}
