#!/usr/bin/env python3
"""make_fixtures.py - T218. Regenerates the analyzer fixture "recordings" (PNG frame sequences) with Pillow. Run on a host that has Pillow; the
generated PNGs are committed, so the analyzers and their self-test need only the Python standard library (+ the tesseract CLI for the OCR leg).
  golden_good/       3 frames of a sign-in screen (text "Sign in", a button), a little motion between frames
  golden_bad/        3 IDENTICAL frames: the expected text is missing and a red error overlay ("ERROR 500") covers the screen (frozen + overlay)
  negative_control/  3 frames of a legitimately DIFFERENT valid screen ("Welcome home", green): must pass its own expectation, never be flagged
Usage: python3 make_fixtures.py [OUTDIR]   (default: ./fixtures next to this file)
"""
import os
import sys

from PIL import Image, ImageDraw, ImageFont

W, H = 480, 270


def font(sz):
    return ImageFont.load_default(size=sz)


def sign_in(shift):
    im = Image.new("RGB", (W, H), (236, 239, 244))
    d = ImageDraw.Draw(im)
    d.text((60, 40), "Sign in", fill=(20, 24, 33), font=font(44))
    d.rectangle((60, 130 + shift, 300, 180 + shift), fill=(46, 90, 220))
    d.text((90, 140 + shift), "Continue", fill=(255, 255, 255), font=font(30))
    return im


def error_overlay():
    im = Image.new("RGB", (W, H), (200, 30, 30))
    d = ImageDraw.Draw(im)
    d.text((50, 100), "ERROR 500", fill=(255, 255, 255), font=font(48))
    return im


def welcome(shift):
    im = Image.new("RGB", (W, H), (222, 240, 224))
    d = ImageDraw.Draw(im)
    d.text((40, 60 + shift), "Welcome home", fill=(18, 60, 28), font=font(44))
    return im


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), "fixtures")
    for name, frames in (("golden_good", [sign_in(0), sign_in(14), sign_in(28)]),
                         ("golden_bad", [error_overlay()] * 3),
                         ("negative_control", [welcome(0), welcome(10), welcome(20)])):
        d = os.path.join(out, name)
        os.makedirs(d, exist_ok=True)
        for i, f in enumerate(frames):
            f.save(os.path.join(d, "frame_%04d.png" % i), optimize=True)


if __name__ == "__main__":
    main()
