"""pngio.py - T218. A minimal PNG reader on the Python standard library (zlib + struct), so the frame analyzers run in any image that has python3.
read_rgb(path) -> (width, height, pixels) where pixels is a flat list of (r, g, b) tuples (alpha dropped, 16-bit reduced to 8-bit, grey and palette expanded).
Supported: non-interlaced PNG, colour types 0, 2, 3, 4, 6, bit depths 1, 2, 4, 8, 16. Anything else raises ValueError (an unreadable frame is a finding, never a skip)."""
import struct
import zlib

SIG = b"\x89PNG\r\n\x1a\n"


def _paeth(a, b, c):
    p = a + b - c
    pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
    if pa <= pb and pa <= pc:
        return a
    return b if pb <= pc else c


def read_rgb(path):
    data = open(path, "rb").read()
    if data[:8] != SIG:
        raise ValueError("not a PNG file")
    pos, idat, plte, hdr = 8, [], None, None
    while pos + 8 <= len(data):
        ln, typ = struct.unpack(">I4s", data[pos:pos + 8])
        body = data[pos + 8:pos + 8 + ln]
        pos += 12 + ln
        if typ == b"IHDR":
            hdr = struct.unpack(">IIBBBBB", body)
        elif typ == b"PLTE":
            plte = [tuple(body[i:i + 3]) for i in range(0, len(body), 3)]
        elif typ == b"IDAT":
            idat.append(body)
        elif typ == b"IEND":
            break
    if hdr is None or not idat:
        raise ValueError("PNG without IHDR/IDAT")
    w, h, depth, ctype, _comp, _filt, interlace = hdr
    if interlace:
        raise ValueError("interlaced PNG is not supported")
    chans = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}.get(ctype)
    if chans is None or depth not in (1, 2, 4, 8, 16):
        raise ValueError("unsupported PNG colour type %d / depth %d" % (ctype, depth))
    raw = zlib.decompress(b"".join(idat))
    bpp = max(1, chans * depth // 8)
    stride = (w * chans * depth + 7) // 8
    if len(raw) < h * (stride + 1):
        raise ValueError("truncated PNG pixel data")
    rows, prev = [], bytearray(stride)
    for y in range(h):
        off = y * (stride + 1)
        f, line = raw[off], bytearray(raw[off + 1:off + 1 + stride])
        if f == 1:
            for i in range(bpp, stride):
                line[i] = (line[i] + line[i - bpp]) & 255
        elif f == 2:
            for i in range(stride):
                line[i] = (line[i] + prev[i]) & 255
        elif f == 3:
            for i in range(stride):
                left = line[i - bpp] if i >= bpp else 0
                line[i] = (line[i] + ((left + prev[i]) >> 1)) & 255
        elif f == 4:
            for i in range(stride):
                left = line[i - bpp] if i >= bpp else 0
                ul = prev[i - bpp] if i >= bpp else 0
                line[i] = (line[i] + _paeth(left, prev[i], ul)) & 255
        elif f != 0:
            raise ValueError("bad PNG filter %d" % f)
        rows.append(line)
        prev = line
    px = []
    for line in rows:
        if depth == 8:
            if ctype == 2:
                px.extend(zip(line[0::3], line[1::3], line[2::3]))
            elif ctype == 6:
                px.extend(zip(line[0::4], line[1::4], line[2::4]))
            elif ctype == 0:
                px.extend((v, v, v) for v in line)
            elif ctype == 4:
                px.extend((v, v, v) for v in line[0::2])
            else:
                if plte is None:
                    raise ValueError("palette PNG without PLTE")
                px.extend(plte[v] for v in line)
        elif depth == 16:
            ch = [line[i] for i in range(0, len(line), 2)]
            if ctype == 2:
                px.extend(zip(ch[0::3], ch[1::3], ch[2::3]))
            elif ctype == 6:
                px.extend(zip(ch[0::4], ch[1::4], ch[2::4]))
            elif ctype == 0:
                px.extend((v, v, v) for v in ch)
            else:
                px.extend((v, v, v) for v in ch[0::2])
        else:  # 1, 2, 4 bit: grey or palette
            vals = []
            mask = (1 << depth) - 1
            for byte in line:
                for s in range(8 - depth, -1, -depth):
                    vals.append((byte >> s) & mask)
            vals = vals[:w]
            if ctype == 3:
                if plte is None:
                    raise ValueError("palette PNG without PLTE")
                px.extend(plte[v] for v in vals)
            else:
                scale = 255 // mask
                px.extend((v * scale,) * 3 for v in vals)
    return w, h, px
