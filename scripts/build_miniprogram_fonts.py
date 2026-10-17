"""Build title font and outlined wordmark from official OFL font sources.
Usage: python scripts/build_miniprogram_fonts.py SOURCE_HAN_MEDIUM.otf BARLOW_BOLD_ITALIC.ttf
Requires fonttools[woff]. Keep original upstream fonts outside the mini-program package.
"""
import sys
from pathlib import Path
from fontTools.ttLib import TTFont
from fontTools import subset
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen

root = Path(__file__).resolve().parents[1]
chars = set(chr(code) for code in range(32, 127))
# GB2312 contains the common simplified Chinese repertoire, beyond sample titles.
for high in range(0xA1, 0xF8):
    for low in range(0xA1, 0xFF):
        try:
            chars.update(bytes([high, low]).decode('gb2312'))
        except UnicodeDecodeError:
            pass
for path in (root / 'miniprogram').rglob('*'):
    if path.suffix in {'.js', '.wxml', '.wxss', '.json'}:
        chars.update(path.read_text())
font = TTFont(sys.argv[1])
options = subset.Options()
options.flavor = 'woff'
options.recommended_glyphs = True
builder = subset.Subsetter(options=options)
builder.populate(text=''.join(sorted(chars)))
builder.subset(font)
font.flavor = 'woff'
# Subsetting modifies the font: avoid the upstream reserved font name 'Source'.
for record in font['name'].names:
    names = {1: 'AnyPitch Serif', 2: 'Medium', 3: 'AnyPitchSerif-Medium',
             4: 'AnyPitch Serif Medium', 6: 'AnyPitchSerif-Medium',
             16: 'AnyPitch Serif', 17: 'Medium'}
    if record.nameID in names:
        record.string = names[record.nameID].encode(record.getEncoding())
if 'CFF ' in font:
    font['CFF '].cff.fontNames = ['AnyPitchSerif-Medium']
    top = font['CFF '].cff.topDictIndex[0]
    top.FamilyName, top.FullName = 'AnyPitch Serif', 'AnyPitch Serif Medium'
output = root / 'web/public/assets/miniprogram/fonts/source-han-serif-medium.woff'
output.parent.mkdir(parents=True, exist_ok=True)
font.save(output)
print(f'{output.name}: {output.stat().st_size} bytes, {len(chars)} characters requested')
font = TTFont(sys.argv[2])
glyphs, cmap = font.getGlyphSet(), font.getBestCmap()
x, paths = 0, []
for character in 'XTC FC':
    name = cmap[ord(character)]
    pen = SVGPathPen(glyphs)
    glyphs[name].draw(TransformPen(pen, (1, 0, 0, -1, x, 720)))
    paths.append(pen.getCommands())
    x += font['hmtx'][name][0]
(root / 'miniprogram/assets/xtc-fc.svg').write_text(
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {x} 750" fill="#203648">'
    f'<path d="{"".join(paths)}"/></svg>\n')
