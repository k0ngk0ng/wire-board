// Bounded reset-code inspection. Runs through Ghidra directly, not REA.
import ghidra.app.script.GhidraScript;
import ghidra.app.cmd.disassemble.DisassembleCommand;
import ghidra.program.model.address.AddressSet;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.listing.InstructionIterator;
import java.nio.file.Files;
import java.nio.file.Path;

public class MdEntryProbe extends GhidraScript {
    public void run() throws Exception {
        String[] args = getScriptArgs();
        Path output = Path.of(args[0]).toAbsolutePath().normalize();
        Path workspace = Path.of(args[1]).toRealPath();
        if (!output.startsWith(workspace)) throw new IllegalArgumentException("Output outside workspace");
        StringBuilder text = new StringBuilder();
        text.append("route=direct-ghidra-headless; REA raw 68000 unsupported\n");
        text.append("language=" + currentProgram.getLanguageID() + "\n");
        text.append("scope=ROM reset code 0x200..0x5ff; not gameplay recovery\n");
        text.append("rom_base=" + currentProgram.getMinAddress() + "\n");
        long reset = Integer.toUnsignedLong(currentProgram.getMemory().getInt(toAddr(4)));
        text.append(String.format("reset_vector=0x%08x\n", reset));
        AddressSet scope = new AddressSet(toAddr(0x200), toAddr(0x5ff));
        new DisassembleCommand(toAddr(reset), scope, true).applyTo(currentProgram, monitor);
        InstructionIterator instructions = currentProgram.getListing().getInstructions(scope, true);
        int count = 0;
        while (instructions.hasNext() && count < 160) {
            Instruction i = instructions.next();
            text.append(i.getAddress() + " " + i + "\n");
            count++;
        }
        text.append("printed_instructions=" + count + "\n");
        Files.writeString(output, text.toString());
        println(text.toString());
    }
}
