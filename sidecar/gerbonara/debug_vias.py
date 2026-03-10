import sys, tarfile, gzip, io, tempfile, logging
logging.disable(logging.CRITICAL)
sys.path.insert(0, '/app')
from main import _parse_features, _find_layer_features, _read_units, _parse_matrix, _matrix_type_to_ltype, _parse_rout
from pathlib import Path

with tempfile.TemporaryDirectory() as tmp:
    with open('/tmp/board1.tgz', 'rb') as f:
        with gzip.open(f, 'rb') as gz:
            inner = io.BytesIO(gz.read())
    with tarfile.open(fileobj=inner, mode='r:*') as tf:
        tf.extractall(tmp)
    tmp = Path(tmp)
    for p in tmp.rglob('matrix/matrix'):
        job_root = p.parent.parent
        break
    step = list((job_root / 'steps').iterdir())[0]
    layers_dir = step / 'layers'
    units = _read_units(step / 'stephdr')
    layer_defs = _parse_matrix(job_root / 'matrix' / 'matrix')

    all_drills = []
    for ld in layer_defs:
        feat = _find_layer_features(layers_dir, ld['name'])
        if feat is None:
            continue
        ltype = _matrix_type_to_ltype(ld['type'])
        if ltype is None:
            continue
        before = len(all_drills)
        traces, pads, vias = [], [], []
        _parse_features(feat, ld['name'], ltype, units, traces, pads, vias, drills=all_drills)
        added = len(all_drills) - before
        if added > 0:
            lname = ld['name']
            ltype_name = ld['type']
            print(f"{lname} ({ltype_name}): +{added} drills")
            for d in all_drills[before:before+3]:
                print(f"   ({d.x:.3f}, {d.y:.3f}) diam={d.diamMM:.4f} plated={d.plated}")

    rout_feat = layers_dir / 'rout' / 'features'
    if rout_feat.exists():
        before_rout = len(all_drills)
        _parse_rout(rout_feat, units, all_drills)
        added_rout = len(all_drills) - before_rout
        print(f"rout: +{added_rout} drills")
        for d in all_drills[before_rout:before_rout+3]:
            print(f"   ({d.x:.3f}, {d.y:.3f}) diam={d.diamMM:.4f} plated={d.plated}")

    print(f"Total drills: {len(all_drills)} (plated={sum(1 for d in all_drills if d.plated)})")
    print()
    print("All unique diameters:", sorted(set(round(d.diamMM, 4) for d in all_drills)))

    # Now check the pad coordinates vs drill coordinates
    all_pads = []
    for ld in layer_defs:
        feat = _find_layer_features(layers_dir, ld['name'])
        if feat is None:
            continue
        ltype = _matrix_type_to_ltype(ld['type'])
        if ltype not in ('COPPER', 'POWER_GROUND'):
            continue
        traces, vias = [], []
        _parse_features(feat, ld['name'], ltype, units, traces, all_pads, vias)

    print(f"Copper pads: {len(all_pads)}")
    print("Sample copper pad coords:", [(round(p.x,3), round(p.y,3), round(p.widthMM,4), p.layer) for p in all_pads[:5]])

    # Try manual correlation for first plated drill
    plated = [d for d in all_drills if d.plated]
    if plated:
        drill = plated[0]
        print(f"\nFirst plated drill: ({drill.x:.3f}, {drill.y:.3f}) diam={drill.diamMM:.4f}")
        nearest = sorted(all_pads, key=lambda p: (drill.x-p.x)**2+(drill.y-p.y)**2)[:3]
        for p in nearest:
            dist = ((drill.x-p.x)**2+(drill.y-p.y)**2)**0.5
            print(f"  dist={dist:.4f} pad=({p.x:.3f},{p.y:.3f}) w={p.widthMM:.4f} layer={p.layer}")
