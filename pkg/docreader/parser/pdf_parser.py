"""
PDF Parser with embedded image extraction.

Uses pypdf to extract text and embedded images page-by-page from PDFs.
Images are placed after the text of the page they belong to, preserving
natural document layout.  Falls back to PDFScannedParser (page renderer)
for image-only / scanned PDFs where no text can be extracted.
"""

import base64
import io
import logging
import os

from pypdf import PdfReader

from docreader.models.document import Document
from docreader.parser.base_parser import BaseParser

logger = logging.getLogger(__name__)

_MIME_MAP = {
    "/DCTDecode": "image/jpeg",
    "/JPXDecode": "image/jp2",
    "/CCITTFaxDecode": "image/tiff",
}


def _guess_mime(image_file: bytes) -> str:
    """Quick MIME sniffing from file header."""
    if image_file.startswith(b'\xff\xd8'):
        return "image/jpeg"
    if image_file.startswith(b'\x89PNG\r\n\x1a\n'):
        return "image/png"
    if image_file[:4] == b'GIF8':
        return "image/gif"
    if image_file[:4] == b'RIFF' and image_file[8:12] == b'WEBP':
        return "image/webp"
    return "application/octet-stream"


def _mime_to_ext(mime: str) -> str:
    return {
        "image/png": ".png",
        "image/jpeg": ".jpg",
        "image/gif": ".gif",
        "image/webp": ".webp",
        "image/bmp": ".bmp",
        "image/tiff": ".tiff",
        "image/jp2": ".jp2",
    }.get(mime, ".png")


class PDFScannedParser(BaseParser):
    """Fallback parser for scanned PDFs.

    If the primary parser extracts no text (e.g. Markitdown on a scanned PDF),
    this parser converts each page into an image. The Go App will then perform
    OCR on the extracted images.
    """

    def parse_into_text(self, content: bytes) -> Document:
        import pdfplumber
        images = {}
        markdown_lines = []

        base_name = os.path.splitext(self.file_name or "document")[0]

        logger.info("PDFScannedParser: Attempting to convert PDF pages to images for %s", self.file_name)

        try:
            with pdfplumber.open(io.BytesIO(content)) as pdf:
                for i, page in enumerate(pdf.pages):
                    img_obj = page.to_image(resolution=150).original
                    img_byte_arr = io.BytesIO()
                    img_obj.save(img_byte_arr, format="PNG")
                    img_bytes = img_byte_arr.getvalue()

                    page_filename = f"{base_name}_page_{i+1}.png"
                    ref_path = f"images/{page_filename}"

                    markdown_lines.append(f"![{page_filename}]({ref_path})")
                    images[ref_path] = base64.b64encode(img_bytes).decode("utf-8")

            text = "\n\n".join(markdown_lines)
            return Document(
                content=text,
                images=images,
                metadata={
                    "image_source_type": "scanned_pdf",
                    "page_count": len(pdf.pages),
                },
            )
        except Exception as e:
            logger.exception("PDFScannedParser failed to parse PDF: %v", e)
            raise e


class PDFParser(BaseParser):
    """PDF Parser: text + embedded images extracted page-by-page via pypdf.

    Falls back to PDFScannedParser (page renderer) when no text or images
    can be extracted (e.g. scanned PDFs).
    """

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._scanned_fallback = PDFScannedParser(*args, **kwargs)

    def parse_into_text(self, content: bytes) -> Document:
        """按页提取文字和嵌入图片，图片放在对应页文字之后。

        不再依赖 markitdown——直接用 pypdf 按页取文字，同时从
        /XObject 抠出嵌入图片。图片引用紧跟在所属页文字后面，
        自然对齐原文位置。
        """
        try:
            reader = PdfReader(io.BytesIO(content))
        except Exception:
            logger.exception("PDFParser: cannot open PDF")
            return self._scanned_fallback.parse_into_text(content)

        images: dict = {}
        seen_hashes: set = set()
        pages_md: list = []
        img_idx = 0

        for page_num, page in enumerate(reader.pages, 1):
            # --- 提取本页文字 ---
            try:
                page_text = page.extract_text() or ""
            except Exception:
                page_text = ""

            # --- 提取本页嵌入图片 ---
            page_images: list = []
            resources = None
            try:
                resources = page.get("/Resources")
            except Exception:
                pass
            if resources is not None:
                xobjects = None
                try:
                    xobjects = resources.get("/XObject")
                except Exception:
                    pass
                if isinstance(xobjects, dict):
                    for obj_name, xobj in xobjects.items():
                        try:
                            subtype = str(xobj.get("/Subtype", "")).strip("/")
                            if subtype != "Image":
                                continue
                        except Exception:
                            continue
                        try:
                            raw = xobj.get_data()
                        except Exception:
                            continue
                        if not raw or len(raw) < 64:
                            continue

                        h = hash(raw)
                        if h in seen_hashes:
                            continue
                        seen_hashes.add(h)

                        mime = "application/octet-stream"
                        try:
                            filters = xobj.get("/Filter", [])
                            filter_name = str(filters[0]).strip("/")
                            mime = _MIME_MAP.get(filter_name, "application/octet-stream")
                        except Exception:
                            pass
                        if mime == "application/octet-stream":
                            mime = _guess_mime(raw)

                        img_idx += 1
                        ext = _mime_to_ext(mime)
                        fname = f"image_{img_idx}{ext}"
                        ref_path = f"images/{fname}"
                        images[ref_path] = base64.b64encode(raw).decode("utf-8")
                        page_images.append(f"![{fname}]({ref_path})")

            # --- 组装本页 markdown ---
            if not page_text.strip() and not page_images:
                continue  # 空页跳过

            block_lines: list = []
            if page_text.strip():
                block_lines.append(page_text.strip())
            if page_images:
                block_lines.extend(page_images)

            if page_text.strip():
                # 有文字：作为独立段落
                pages_md.append("\n\n".join(block_lines))
            elif pages_md:
                # 纯图片页（封面、图表等）：追加到上一段，避免 splitter 切出无文本分片
                pages_md[-1] += "\n\n" + "\n\n".join(block_lines)
            else:
                # 文档第一页就是纯图片（罕见）
                pages_md.append("\n\n".join(block_lines))

        if not pages_md:
            logger.info("PDFParser: no text or images extracted, trying scanned fallback")
            return self._scanned_fallback.parse_into_text(content)

        text = "\n\n".join(pages_md)
        logger.info(
            "PDFParser: pages=%d chars=%d images=%d",
            len(pages_md), len(text), len(images),
        )
        return Document(
            content=text,
            images=images,
            metadata={
                "image_source_type": "embedded",
                "image_count": str(len(images)),
                "page_count": str(len(pages_md)),
            },
        )
