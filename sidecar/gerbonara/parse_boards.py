import sys
sys.path.insert(0, '/app')
from main import parse_odb

boards = [
    ('board1', '/tmp/board1.tgz'),
    ('board2', '/tmp/board2.tgz'),
    ('rigidflex', '/tmp/rigidflex.tgz'),
]

for label, path in boards:
    b = parse_odb(path)
    pad_widths = sorted(set(round(p.widthMM, 3) for p in b.pads))[:8]
    via_outers = sorted(set(round(v.outerDiamMM, 3) for v in b.vias))[:5]
    big_pads = [round(p.widthMM, 2) for p in b.pads if p.widthMM > 10][:5]
    print(f'=== {label} ===')
    print(f'  layers:  {len(b.layers)}')
    for l in b.layers[:8]:
        print(f'    {l.name} ({l.type})')
    print(f'  traces:  {len(b.traces)}')
    print(f'  pads:    {len(b.pads)}  sample widths mm: {pad_widths}')
    if big_pads:
        print(f'  LARGE PADS (>10mm): {big_pads}  <- possible unit bug')
    print(f'  vias:    {len(b.vias)}  sample outers mm: {via_outers}')
    print(f'  drills:  {len(b.drills)}')
    print(f'  outline: {len(b.outline)} pts')
    print(f'  warnings: {len(b.warnings)}')
    for w in b.warnings[:5]:
        print(f'    {w}')
    print()
