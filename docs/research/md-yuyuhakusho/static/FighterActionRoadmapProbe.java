// Research-only bounded original instruction and flow export. No target execution.
import ghidra.app.script.GhidraScript;
import ghidra.app.cmd.disassemble.DisassembleCommand;
import ghidra.program.model.address.Address;
import ghidra.program.model.address.AddressSet;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.listing.InstructionIterator;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.TreeSet;

public class FighterActionRoadmapProbe extends GhidraScript {
    private String quote(String value) { return "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\""; }
    public void run() throws Exception {
        String[] args = getScriptArgs();
        Path root = Path.of(args[0]).toRealPath();
        Path output = Path.of(args[1]).toAbsolutePath().normalize();
        if (!output.startsWith(root)) throw new IllegalArgumentException("Outside worktree");
        AddressSet scope = new AddressSet(toAddr(0x15f4a), toAddr(0x1cc81));
        TreeSet<Long> pending = new TreeSet<>();
        for (String seed : args[2].split(",")) pending.add(Long.parseUnsignedLong(seed, 16));
        HashSet<Long> seeded = new HashSet<>();
        while (!pending.isEmpty()) {
            ArrayList<Long> batch = new ArrayList<>(pending); pending.clear();
            for (long seed : batch) {
                if (!seeded.add(seed)) continue;
                if (!scope.contains(toAddr(seed))) throw new IllegalArgumentException("Seed outside declared scope");
                new DisassembleCommand(toAddr(seed), scope, true).applyTo(currentProgram, monitor);
            }
            InstructionIterator scan = currentProgram.getListing().getInstructions(scope, true);
            while (scan.hasNext()) for (Address target : scan.next().getFlows()) {
                long value = target.getOffset();
                if (scope.contains(target) && !seeded.contains(value)) pending.add(value);
            }
        }
        StringBuilder json = new StringBuilder("{\"route\":\"direct Ghidra headless; REA raw 68000 unsupported\",\"scope\":[\"0x15f4a\",\"0x1cc81\"],\"instructions\":[");
        InstructionIterator instructions = currentProgram.getListing().getInstructions(scope, true);
        boolean comma = false;
        while (instructions.hasNext()) {
            Instruction instruction = instructions.next();
            if (comma) json.append(","); comma = true;
            json.append("{\"pc\":").append(quote(instruction.getAddress().toString()));
            json.append(",\"length\":").append(instruction.getLength());
            json.append(",\"text\":").append(quote(instruction.toString()));
            StringBuilder bytes = new StringBuilder();
            for (byte value : instruction.getBytes()) bytes.append(String.format("%02x", value & 255));
            json.append(",\"bytes\":").append(quote(bytes.toString()));
            json.append(",\"flow_kind\":").append(quote(instruction.getFlowType().toString()));
            Address fallthrough = instruction.getFallThrough();
            json.append(",\"fallthrough\":").append(fallthrough == null ? "null" : quote(fallthrough.toString()));
            json.append(",\"flows\":["); boolean flowComma = false;
            for (Address target : instruction.getFlows()) {
                if (flowComma) json.append(","); flowComma = true;
                json.append(quote(target.toString()));
            }
            json.append("]}");
        }
        json.append("]}\n");
        Files.writeString(output, json.toString());
        println("Wrote bounded action flow to " + output);
    }
}
