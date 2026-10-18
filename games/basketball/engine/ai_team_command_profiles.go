// Original bank5 command subrows at9467 and mode flags at95EF.
// Source bytes and SHA are in testdata/team-commands-source.json.
package engine

var originalHumanCommandPlans = [6][12]AIMode{{2, 2, 17, 8, 36, 36, 24, 8, 37, 37, 23, 8}, {2, 2, 17, 8, 36, 36, 24, 8, 37, 37, 23, 8}, {12, 12, 17, 12, 36, 36, 11, 12, 30, 30, 23, 12}, {2, 2, 17, 8, 36, 36, 11, 8, 13, 13, 25, 8}, {2, 2, 17, 32, 36, 36, 11, 8, 38, 38, 23, 8}, {5, 5, 17, 5, 36, 36, 11, 5, 37, 37, 23, 5}}
var originalCommandModeFlags = [42]uint8{0, 16, 1, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 16, 0, 16, 0, 0, 16, 0, 0, 0, 0, 1, 32, 32}
