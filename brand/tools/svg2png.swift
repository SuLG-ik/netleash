// Renders an SVG to a PNG with transparency using AppKit (CoreSVG). Usage: svg2png in.svg out.png <pixel-width>
import AppKit

let a = CommandLine.arguments
guard a.count == 4, let w = Int(a[3]), let img = NSImage(contentsOfFile: a[1]) else {
    fputs("usage: svg2png in.svg out.png width\n", stderr); exit(1)
}
let ratio = img.size.height / img.size.width
let h = Int((Double(w) * Double(ratio)).rounded())
guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: w, pixelsHigh: h, bitsPerSample: 8, samplesPerPixel: 4,
                                 hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { exit(2) }
rep.size = NSSize(width: w, height: h)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSColor.clear.set(); NSRect(x: 0, y: 0, width: w, height: h).fill()
img.draw(in: NSRect(x: 0, y: 0, width: w, height: h), from: .zero, operation: .sourceOver, fraction: 1)
NSGraphicsContext.restoreGraphicsState()
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: a[2]))
