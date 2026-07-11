"""Generic HTML(Jinja2) → PDF(WeasyPrint) rendering. Document-agnostic."""

from __future__ import annotations

import pathlib

from jinja2 import Environment, FileSystemLoader, select_autoescape

from py_dfe.exceptions import DANFE_RENDER_FAILED, DFeError

_TEMPLATE_DIR = pathlib.Path(__file__).parent / "templates"

# CSS px → mm (CSS px is 1/96 inch).
_PX_TO_MM = 25.4 / 96.0
# Extra height added to the fitted page so content never spills to a new page.
_HEIGHT_SLACK_MM = 2.0

_env = Environment(
    loader=FileSystemLoader(str(_TEMPLATE_DIR)),
    autoescape=select_autoescape(["html", "xml"]),
)


def render_html(template_name: str, context: dict) -> str:
    """Render a Jinja2 template (from the danfe/templates/ dir) to an HTML string."""
    template = _env.get_template(template_name)
    return template.render(**context)


def html_to_pdf(html: str) -> bytes:
    """Convert a single HTML string to a height-fitted PDF."""
    return htmls_to_pdf([html])


def htmls_to_pdf(pages: list[str], *, fit_height: bool = True) -> bytes:
    """Render HTML strings to one merged PDF.

    fit_height=True  → each page measured and snugged to its content height
                       (thermal / roll: NFC-e, DANFE simplificado/etiqueta).
    fit_height=False → honor each document's own ``@page`` size and let it
                       paginate naturally (fixed A4: DANFE retrato/paisagem).
    """
    try:
        from weasyprint import HTML  # imported lazily (heavy native deps)

        base = str(_TEMPLATE_DIR)
        if fit_height:
            docs = [_render_fitted(HTML, base, html) for html in pages]
        else:
            docs = [HTML(string=html, base_url=base).render() for html in pages]
        if not docs:
            return HTML(string="", base_url=base).write_pdf()
        merged_pages = [page for doc in docs for page in doc.pages]
        return docs[0].copy(merged_pages).write_pdf()
    except DFeError:
        raise
    except Exception as exc:  # noqa: BLE001 - wrap everything as DFeError
        raise DFeError(500, DANFE_RENDER_FAILED, f"PDF render failed: {exc}") from exc


def _render_fitted(html_cls, base: str, html: str):
    """Render *html*, then re-render with a page height fitted to its content."""
    doc = html_cls(string=html, base_url=base).render()
    if not doc.pages:
        return doc
    height_mm = (
        max(_content_height_px(page) for page in doc.pages) * _PX_TO_MM
        + _HEIGHT_SLACK_MM
    )
    width_mm = doc.pages[0].width * _PX_TO_MM
    override = f"<style>@page{{size:{width_mm:.2f}mm {height_mm:.2f}mm}}</style>"
    final_html = (
        html.replace("</body>", override + "</body>")
        if "</body>" in html
        else html + override
    )
    return html_cls(string=final_html, base_url=base).render()


def _content_height_px(page) -> float:
    """Height in px from page top to the bottom of the deepest content box."""
    page_box = page._page_box
    deepest = max(
        (_deepest_bottom(child) for child in (page_box.children or [])),
        default=page_box.position_y,
    )
    return deepest + (page_box.margin_bottom or 0)


def _deepest_bottom(box) -> float:
    """Lowest border-box edge (position_y + height) across *box* and descendants."""
    height = box.height if isinstance(box.height, (int, float)) else 0
    bottom = box.position_y + height
    for child in getattr(box, "children", None) or []:
        bottom = max(bottom, _deepest_bottom(child))
    return bottom
