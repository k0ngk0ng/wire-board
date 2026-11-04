import ghidra.app.script.GhidraScript;
import ghidra.app.cmd.disassemble.DisassembleCommand;
import ghidra.program.model.address.AddressSet;
import ghidra.program.model.listing.InstructionIterator;
import java.io.ByteArrayInputStream;
import java.nio.file.Files;
import java.nio.file.Path;

public class MdRamFrameProbe extends GhidraScript {
    public void run() throws Exception {
        String[] args = getScriptArgs();
        Path root = Path.of(args[0]).toRealPath();
        Path input = Path.of(args[1]).toRealPath();
        Path output = Path.of(args[2]).toAbsolutePath().normalize();
        if (!input.startsWith(root) || !output.startsWith(root)) throw new IllegalArgumentException("Outside worktree");
        byte[] data = Files.readAllBytes(input);
        currentProgram.getMemory().createInitializedBlock("observed_frame_irq", toAddr(0xfff000), new ByteArrayInputStream(data), data.length, monitor, false);
        AddressSet scope = new AddressSet(toAddr(0xfff000), toAddr(0xfff000 + data.length - 1));
        new DisassembleCommand(toAddr(0xfff000), scope, true).applyTo(currentProgram, monitor);
        StringBuilder result = new StringBuilder("Observed2400-frame RAM code; direct Ghidra68000; static instructions only\n");
        InstructionIterator instructions = currentProgram.getListing().getInstructions(scope, true);
        while (instructions.hasNext()) {
            var instruction = instructions.next();
            result.append(instruction.getAddress()).append(" ").append(instruction).append("\n");
        }
        Files.writeString(output, result.toString());
    }
}
