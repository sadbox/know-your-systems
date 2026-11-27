# /// script
# dependencies = ["pillow", "numpy"]
# ///
"""Prepare the cover illustration for print.

Input : art/groundhog-source.png  (black ink on a transparent background)
Output: assets/groundhog-cover.png  ink only, as black with transparency, cropped to the drawing and
                                    enlarged so edges stay crisp at 4.6 in wide. Unprinted areas are
                                    transparent, so the cover stock shows through.
        assets/groundhog-spot.png   the animal alone, softly faded at the edges, in the inside-page green.
        assets/site/favicon.png, apple-touch.png   the spot on the cover colour, for the website
Run   : uv run art/prep_art.py
"""
import os

import numpy as np
from PIL import Image

here = os.path.dirname(os.path.abspath(__file__))
src = Image.open(os.path.join(here, 'groundhog-source.png')).convert('RGBA')
white = Image.new('RGBA', src.size, (255, 255, 255, 255))
gray = Image.alpha_composite(white, src).convert('L')            # what it looks like on white paper

def ink(img, scale, colour):
    """Enlarge, firm up the edges, and return colour + alpha where alpha is the amount of ink."""
    big = img.resize((img.width * scale, img.height * scale), Image.LANCZOS)
    a = 255.0 - np.asarray(big, dtype=np.float32)
    a = np.clip((a - 96.0) * 3.2 + 128.0, 0, 255)                # steeper edge, still anti-aliased
    out = np.zeros((big.height, big.width, 4), np.uint8)
    out[..., 0], out[..., 1], out[..., 2] = colour
    out[..., 3] = a.astype(np.uint8)
    return out

# cover: the whole drawing, cropped to the rows that carry ink
g = np.asarray(gray)
rows = np.where((g < 128).sum(axis=1) > 0)[0]
top, bottom = max(rows.min() - 14, 0), min(rows.max() + 14, g.shape[0])
cover = ink(gray.crop((0, top, gray.width, bottom)), 2, (0, 0, 0))
Image.fromarray(cover).save(os.path.join(here, '..', 'assets', 'groundhog-cover.png'), optimize=True)

# spot: the animal and the grass around it, fading out in an oval
box = (250, 560, 800, 1090)
spot = ink(gray.crop(box), 2, (0x1f, 0x5a, 0x3c))
h, w = spot.shape[:2]
yy, xx = np.mgrid[0:h, 0:w]
r = np.sqrt(((xx - w / 2) / (w / 2)) ** 2 + ((yy - h * 0.52) / (h / 2)) ** 2)
fade = np.clip((1.0 - r) / 0.28, 0, 1)
spot[..., 3] = (spot[..., 3] * fade).astype(np.uint8)
Image.fromarray(spot).save(os.path.join(here, '..', 'assets', 'groundhog-spot.png'), optimize=True)
print('cover', cover.shape[1], 'x', cover.shape[0], ' spot', w, 'x', h)

# website icons: the spot on the cover colour, square
os.makedirs(os.path.join(here, '..', 'assets', 'site'), exist_ok=True)
sp = Image.fromarray(spot)
side = max(sp.size)
for name, px in (('favicon.png', 64), ('apple-touch.png', 180)):
    tile = Image.new('RGBA', (side, side), (0xf9, 0xc9, 0x95, 255))
    tile.alpha_composite(sp, ((side - sp.width) // 2, (side - sp.height) // 2))
    tile.convert('RGB').resize((px, px), Image.LANCZOS).save(os.path.join(here, '..', 'assets', 'site', name), optimize=True)
