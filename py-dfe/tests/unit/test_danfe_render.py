import pytest

from py_dfe.danfe.render import render_html, html_to_pdf


def test_render_html_substitutes_context():
    out = render_html("_probe.html", {"name": "Mundo"})
    assert "Olá Mundo" in out


def test_html_to_pdf_returns_pdf_bytes():
    pytest.importorskip("weasyprint")  # needs native libs
    pdf = html_to_pdf("<html><body><p>x</p></body></html>")
    assert pdf[:4] == b"%PDF"


def test_fixed_size_paginates_a4():
    pytest.importorskip("weasyprint")
    import io as _io

    from pypdf import PdfReader

    from py_dfe.danfe.render import htmls_to_pdf

    # Two A4 pages forced via CSS; fit_height=False must keep both.
    html = """<html><head><style>
      @page { size: A4 portrait; margin: 5mm; }
      .pg { page-break-after: always; height: 280mm; }
    </style></head><body>
      <div class="pg">PAGE ONE</div>
      <div class="pg">PAGE TWO</div>
    </body></html>"""
    pdf = htmls_to_pdf([html], fit_height=False)
    assert pdf[:4] == b"%PDF"
    reader = PdfReader(_io.BytesIO(pdf))
    assert len(reader.pages) == 2


def test_fit_height_default_unchanged():
    pytest.importorskip("weasyprint")
    from py_dfe.danfe.render import htmls_to_pdf
    pdf = htmls_to_pdf(["<html><body><p>hi</p></body></html>"])
    assert pdf[:4] == b"%PDF"
