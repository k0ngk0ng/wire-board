// Bounded static control-flow investigation; no target execution.
import ghidra.app.script.GhidraScript;
import ghidra.app.cmd.disassemble.DisassembleCommand;
import ghidra.program.model.address.AddressSet;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.listing.InstructionIterator;
import java.nio.file.Files;
import java.nio.file.Path;

public class MdControlProbe extends GhidraScript {
    public void run() throws Exception {
        String[] args = getScriptArgs();
        Path output = Path.of(args[0]).toAbsolutePath().normalize();
        Path workspace = Path.of(args[1]).toRealPath();
        if (!output.startsWith(workspace)) throw new IllegalArgumentException("Output outside workspace");
        StringBuilder text = new StringBuilder();
        text.append("route=direct-ghidra-headless; REA raw 68000 unsupported\n");
        text.append("language=" + currentProgram.getLanguageID() + "\n");
        text.append("source=MD Yu Yu Hakusho Japan; static reachable instructions only\n");
        for (int n = 2; n < args.length; n++) {
            String[] spec = args[n].split(":");
            long start = Long.parseUnsignedLong(spec[1], 16);
            long end = Long.parseUnsignedLong(spec[2], 16);
            AddressSet scope = new AddressSet(toAddr(start), toAddr(end));
            for (String value : spec[0].split(",")) {
                long seed = Long.parseUnsignedLong(value, 16);
                new DisassembleCommand(toAddr(seed), scope, true).applyTo(currentProgram, monitor);
            }
            text.append(String.format("\nseeds=%s scope=0x%x..0x%x\n", spec[0], start, end));
            InstructionIterator instructions = currentProgram.getListing().getInstructions(scope, true);
            int count = 0;
            while (instructions.hasNext()) {
                Instruction i = instructions.next();
                text.append(i.getAddress() + " " + i + "\n");
                count++;
            }
            text.append("printed_instructions=" + count + "\n");
        }
        Files.writeString(output, text.toString());
        println("Wrote " + output);
    }
}
