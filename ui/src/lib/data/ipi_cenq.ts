/**
 * Código de Enquadramento Legal do IPI (`IPI/cEnq`). Fonte: Nota Técnica
 * 2020.002 v1.01 (agosto/2022), anexo "Tabela do Código de Enquadramento do
 * IPI", que consolida o anexo XIV da NT 2015.002 e a NT 2016.001.
 *
 * O CST do IPI decide a faixa válida (regras de validação W16-10, rejeições 387
 * e 388): imunidade 001–099, suspensão 101–199, isenção 301–399, e os demais
 * casos usam 999 ou a faixa de redução 601–608. `cEnqOptionsForCst` aplica essa
 * regra — oferecer os 132 códigos em qualquer CST é oferecer 130 rejeições.
 */

export type CEnqGroup = 'imunidade' | 'suspensao' | 'isencao' | 'reducao' | 'outros'

export interface CEnqEntry {
  code: string
  group: CEnqGroup
  description: string
}

export const IPI_CENQ: readonly CEnqEntry[] = [
  {code: "001", group: "imunidade", description: "Livros, jornais, periódicos e o papel destinado à sua impressão - Art. 18 Inciso I do Decreto 7.212/2010"},
  {code: "002", group: "imunidade", description: "Produtos industrializados destinados ao exterior - Art. 18 Inciso II do Decreto 7.212/2010"},
  {code: "003", group: "imunidade", description: "Ouro, definido em lei como ativo financeiro ou instrumento cambial - Art. 18 Inciso III do Decreto 7.212/2010"},
  {code: "004", group: "imunidade", description: "Energia elétrica, derivados de petróleo, combustíveis e minerais do País - Art. 18 Inciso IV do Decreto 7.212/2010"},
  {code: "005", group: "imunidade", description: "Exportação de produtos nacionais - sem saída do território brasileiro - venda para empresa sediada no exterior -atividades de pesquisa ou lavra de jazidas de petróleo e de gás natural - Art. 19 Inciso I do Decreto 7.212/2010"},
  {code: "006", group: "imunidade", description: "Exportação de produtos nacionais - sem saída do território brasileiro - venda para empresa sediada no exterior - incorporados a produto final exportado para o Brasil - Art. 19 Inciso II do Decreto 7.212/2010"},
  {code: "007", group: "imunidade", description: "Exportação de produtos nacionais - sem saída do território brasileiro - venda para órgão ou entidade de governo estrangeiro ou organismo internacional de que o Brasil seja membro, para ser entregue, no País, à ordem do comprador - Art. 19 Inciso III do Decreto 7.212/2010"},
  {code: "101", group: "suspensao", description: "Óleo de menta em bruto, produzido por lavradores - Art. 43 Inciso I do Decreto 7.212/2010"},
  {code: "102", group: "suspensao", description: "Produtos remetidos à exposição em feiras de amostras e promoções semelhantes - Art. 43 Inciso II do Decreto 7.212/2010"},
  {code: "103", group: "suspensao", description: "Produtos remetidos a depósitos fechados ou armazéns-gerais, bem assim aqueles devolvidos ao remetente - Art. 43 Inciso III do Decreto 7.212/2010"},
  {code: "104", group: "suspensao", description: "Produtos industrializados, que com matérias-primas (MP), produtos intermediários (PI) e material de embalagem (ME) importados submetidos a regime aduaneiro especial (drawback - suspensão/isenção), remetidos diretamente a empresas industriais exportadoras - Art. 43 Inciso IV do Decreto 7.212/2010"},
  {code: "105", group: "suspensao", description: "Produtos, destinados à exportação, que saiam do estabelecimento industrial para empresas comerciais exportadoras, com o fim específico de exportação - Art. 43, Inciso V, alínea \"a\" do Decreto 7.212/2010"},
  {code: "106", group: "suspensao", description: "Produtos, destinados à exportação, que saiam do estabelecimento industrial para recintos alfandegados onde se processe o despacho aduaneiro de exportação - Art. 43, Inciso V, alíneas \"b\" do Decreto 7.212/2010"},
  {code: "107", group: "suspensao", description: "Produtos, destinados à exportação, que saiam do estabelecimento industrial para outros locais onde se processe o despacho aduaneiro de exportação - Art. 43, Inciso V, alíneas \"c\" do Decreto 7.212/2010"},
  {code: "108", group: "suspensao", description: "Matérias-primas (MP), produtos intermediários (PI) e material de embalagem (ME) destinados ao executor de industrialização por encomenda - Art. 43 Inciso VI do Decreto 7.212/2010"},
  {code: "109", group: "suspensao", description: "Produtos industrializados por encomenda remetidos ao estabelecimento de origem - Art. 43 Inciso VII do Decreto 7.212/2010"},
  {code: "110", group: "suspensao", description: "Matérias-primas ou produtos intermediários remetidos para emprego em operação industrial realizada pelo remetente fora do estabelecimento - Art. 43 Inciso VIII do Decreto 7.212/2010"},
  {code: "111", group: "suspensao", description: "Veículo, aeronave ou embarcação destinados a emprego em provas de engenharia pelo fabricante - Art. 43 Inciso IX do Decreto 7.212/2010"},
  {code: "112", group: "suspensao", description: "Produtos remetidos, para industrialização ou comércio, de um para outro estabelecimento da mesma firma - Art. 43 Inciso X do Decreto 7.212/2010"},
  {code: "113", group: "suspensao", description: "Bens do ativo permanente remetidos a outro estabelecimento da mesma firma, para serem utilizados no processo industrial do recebedor - Art. 43 Inciso XI do Decreto 7.212/2010"},
  {code: "114", group: "suspensao", description: "Bens do ativo permanente remetidos a outro estabelecimento, para serem utilizados no processo industrial de produtos encomendados pelo remetente - Art. 43 Inciso XII do Decreto 7.212/2010"},
  {code: "115", group: "suspensao", description: "Partes e peças destinadas ao reparo de produtos com defeito de fabricação, quando a operação for executada gratuitamente, em virtude de garantia - Art. 43 Inciso XIII do Decreto 7.212/2010"},
  {code: "116", group: "suspensao", description: "Matérias-primas (MP), produtos intermediários (PI) e material de embalagem (ME) de fabricação nacional, vendidos a estabelecimento industrial, para industrialização de produtos destinados à exportação ou a estabelecimento comercial, para industrialização em outro estabelecimento da mesma firma ou de terceiro, de produto destinado à exportação - Art. 43 Inciso XIV do Decreto 7.212/2010"},
  {code: "117", group: "suspensao", description: "Produtos para emprego ou consumo na industrialização ou elaboração de produto a ser exportado, adquiridos no mercado interno ou importados - Art. 43 Inciso XV do Decreto 7.212/2010"},
  {code: "118", group: "suspensao", description: "Bebidas alcóolicas e demais produtos de produção nacional acondicionados em recipientes de capacidade superior ao limite máximo permitido para venda a varejo - Art. 44 do Decreto 7.212/2010"},
  {code: "119", group: "suspensao", description: "Produtos classificados NCM 21.06.90.10 Ex 02, 22.01, 22.02, exceto os Ex 01 e Ex 02 do Código 22.02.90.00 e 22.03 saídos de estabelecimento industrial destinado a comercial equiparado a industrial - Art. 45 Inciso I do Decreto 7.212/2010"},
  {code: "120", group: "suspensao", description: "Produtos classificados NCM 21.06.90.10 Ex 02, 22.01, 22.02, exceto os Ex 01 e Ex 02 do Código 22.02.90.00 e 22.03 saídos de estabelecimento comercial equiparado a industrial destinado a equiparado a industrial - Art. 45 Inciso II do Decreto 7.212/2010"},
  {code: "121", group: "suspensao", description: "Produtos classificados NCM 21.06.90.10 Ex 02, 22.01, 22.02, exceto os Ex 01 e Ex 02 do Código 22.02.90.00 e 22.03 saídos de importador destinado a equiparado a industrial - Art. 45 Inciso III do Decreto 7.212/2010"},
  {code: "122", group: "suspensao", description: "Matérias-primas (MP), produtos intermediários (PI) e material de embalagem (ME) destinados a estabelecimento que se dedique à elaboração de produtos classificados nos códigos previstos no art. 25 da Lei 10.684/2003 - Art. 46 Inciso I do Decreto 7.212/2010"},
  {code: "123", group: "suspensao", description: "Matérias-primas (MP), produtos intermediários (PI) e material de embalagem (ME) adquiridos por estabelecimentos industriais fabricantes de partes e peças destinadas a estabelecimento industrial fabricante de produto classificado no Capítulo 88 da Tipi - Art. 46 Inciso II do Decreto 7.212/2010"},
  {code: "124", group: "suspensao", description: "Matérias-primas (MP), produtos intermediários (PI) e material de embalagem (ME) adquiridos por pessoas jurídicas preponderantemente exportadoras - Art. 46 Inciso III do Decreto 7.212/2010"},
  {code: "125", group: "suspensao", description: "Materiais e equipamentos destinados a embarcações pré-registradas ou registradas no Registro Especial Brasileira - REB quando adquiridos por estaleiros navais brasileiros - Art. 46 Inciso IV do Decreto 7.212/2010"},
  {code: "126", group: "suspensao", description: "Aquisição por beneficiário de regime aduaneiro suspensivo do imposto, destinado a industrialização para exportação - Art. 47 do Decreto 7.212/2010"},
  {code: "127", group: "suspensao", description: "Desembaraço de produtos de procedência estrangeira importados por lojas francas - Art. 48 Inciso I do Decreto 7.212/2010"},
  {code: "128", group: "suspensao", description: "Desembaraço de maquinas, equipamentos, veículos, aparelhos e instrumentos sem similar nacional importados por empresas nacionais de engenharia, destinados à execução de obras no exterior - Art. 48 Inciso II do Decreto 7.212/2010"},
  {code: "129", group: "suspensao", description: "Desembaraço de produtos de procedência estrangeira com saída de repartições aduaneiras com suspensão do Imposto de Importação - Art. 48 Inciso III do Decreto 7.212/2010"},
  {code: "130", group: "suspensao", description: "Desembaraço de matérias-primas, produtos intermediários e materiais de embalagem, importados diretamente por estabelecimento de que tratam os incisos I a III do caput do Decreto 7.212/2010 - Art. 48 Inciso IV do Decreto 7.212/2010"},
  {code: "131", group: "suspensao", description: "Remessa de produtos para a ZFM destinados ao seu consumo interno, utilização ou industrialização - Art. 84 do Decreto 7.212/2010"},
  {code: "132", group: "suspensao", description: "Remessa de produtos para a ZFM destinados à exportação - Art. 85 Inciso I do Decreto 7.212/2010"},
  {code: "133", group: "suspensao", description: "Produtos que, antes de sua remessa à ZFM, forem enviados pelo seu fabricante a outro estabelecimento, para industrialização adicional, por conta e ordem do destinatário - Art. 85 Inciso II do Decreto 7.212/2010"},
  {code: "134", group: "suspensao", description: "Desembaraço de produtos de procedência estrangeira importados pela ZFM quando ali consumidos ou utilizados, exceto armas, munições, fumo, bebidas alcoólicas e automóveis de passageiros. - Art. 86 do Decreto 7.212/2010"},
  {code: "135", group: "suspensao", description: "Remessa de produtos para a Amazônia Ocidental destinados ao seu consumo interno ou utilização - Art. 96 do Decreto 7.212/2010"},
  {code: "136", group: "suspensao", description: "Entrada de produtos estrangeiros na Área de Livre Comércio de Tabatinga - ALCT destinados ao seu consumo interno ou utilização - Art. 106 do Decreto 7.212/2010"},
  {code: "137", group: "suspensao", description: "Entrada de produtos estrangeiros na Área de Livre Comércio de Guajará-Mirim - ALCGM destinados ao seu consumo interno ou utilização - Art. 109 do Decreto 7.212/2010"},
  {code: "138", group: "suspensao", description: "Entrada de produtos estrangeiros nas Áreas de Livre Comércio de Boa Vista - ALCBV e Bomfim - ALCB destinados a seu consumo interno ou utilização - Art. 112 do Decreto 7.212/2010"},
  {code: "139", group: "suspensao", description: "Entrada de produtos estrangeiros na Área de Livre Comércio de Macapá e Santana - ALCMS destinados a seu consumo interno ou utilização - Art. 116 do Decreto 7.212/2010"},
  {code: "140", group: "suspensao", description: "Entrada de produtos estrangeiros nas Áreas de Livre Comércio de Brasiléia - ALCB e de Cruzeiro do Sul - ALCCS destinados a seu consumo interno ou utilização - Art. 119 do Decreto 7.212/2010"},
  {code: "141", group: "suspensao", description: "Remessa para Zona de Processamento de Exportação - ZPE - Art. 121 do Decreto 7.212/2010"},
  {code: "142", group: "suspensao", description: "Setor Automotivo - Desembaraço aduaneiro, chassis e outros - regime aduaneiro especial - industrialização 87.01 a 87.05 - Art. 136, I do Decreto 7.212/2010"},
  {code: "143", group: "suspensao", description: "Setor Automotivo - Do estabelecimento industrial produtos 87.01 a 87.05 da TIPI - mercado interno - empresa comercial atacadista controlada por PJ encomendante do exterior. - Art. 136, II do Decreto 7.212/2010"},
  {code: "144", group: "suspensao", description: "Setor Automotivo - Do estabelecimento industrial - chassis e outros classificados nas posições 84.29, 84.32, 84.33, 87.01 a 87.06 e 87.11 da TIPI. - Art. 136, III do Decreto 7.212/2010"},
  {code: "145", group: "suspensao", description: "Setor Automotivo - Desembaraço aduaneiro, chassis e outros classificados nas posições 84.29, 84.32, 84.33, 87.01 a 87.06 e 87.11 da TIPI quando importados diretamente por estabelecimento industrial - Art. 136, IV do Decreto 7.212/2010"},
  {code: "146", group: "suspensao", description: "Setor Automotivo - do estabelecimento industrial matérias-primas, os produtos intermediários e os materiais de embalagem, adquiridos por fabricantes, preponderantemente, de componentes, chassis e outros classificados nos Códigos 84.29, 8432.40.00, 8432.80.00, 8433.20, 8433.30.00, 8433.40.00, 8433.5 e 87.01 a 87.06 da TIPI- Art. 136, V do Decreto 7.212/2010"},
  {code: "147", group: "suspensao", description: "Setor Automotivo -Desembaraço aduaneiro, as matérias-primas, os produtos intermediários e os materiais de embalagem, importados diretamente por fabricantes, preponderantemente, de componentes, chassis e outros classificados nos Códigos 84.29, 8432.40.00, 8432.80.00, 8433.20, 8433.30.00, 8433.40.00, 8433.5 e 87.01 a 87.06 da TIPI -Art. 136, VI do Decreto 7.212/2010"},
  {code: "148", group: "suspensao", description: "Bens de Informática e Automação- matérias-primas, os produtos intermediários e os materiais de embalagem, quando adquiridos por estabelecimentos industriais fabricantes dos referidos bens. - Art. 148 do Decreto 7.212/2010"},
  {code: "149", group: "suspensao", description: "Reporto - Saída de Estabelecimento de máquinas e outros quando adquiridos por beneficiários do REPORTO - Art. 166, I do Decreto 7.212/2010"},
  {code: "150", group: "suspensao", description: "Reporto - Desembaraço aduaneiro de máquinas e outros quando adquiridos por beneficiários do REPORTO - Art. 166, II do Decreto 7.212/2010"},
  {code: "151", group: "suspensao", description: "Repes - Desembaraço aduaneiro - bens sem similar nacional importados por beneficiários do REPES - Art. 171 do Decreto 7.212/2010"},
  {code: "152", group: "suspensao", description: "Recine - Saída para beneficiário do regime - Art. 14, III da Lei 12.599/2012"},
  {code: "153", group: "suspensao", description: "Recine - Desembaraço aduaneiro por beneficiário do regime - Art. 14, IV da Lei 12.599/2012"},
  {code: "154", group: "suspensao", description: "Reif - Saída para beneficiário do regime - Lei 12.794/1013, art. 8, III"},
  {code: "155", group: "suspensao", description: "Reif - Desembaraço aduaneiro por beneficiário do regime - Lei 12.794/1013, art. 8, IV"},
  {code: "156", group: "suspensao", description: "Repnbl-Redes - Saída para beneficiário do regime - Lei n° 12.715/2012, art. 30, II"},
  {code: "157", group: "suspensao", description: "Recompe - Saída de matérias-primas e produtos intermediários para beneficiários do regime - Decreto n° 7.243/2010, art. 5°, I"},
  {code: "158", group: "suspensao", description: "Recompe - Saída de matérias-primas e produtos intermediários destinados a industrialização de equipamentos - Programa Estímulo Universidade-Empresa - Apoio à Inovação - Decreto n° 7.243/2010, art. 5°, III"},
  {code: "159", group: "suspensao", description: "Rio 2016 - Produtos nacionais, duráveis, uso e consumo dos eventos, adquiridos pelas pessoas jurídicas mencionadas no § 2o do art. 4o da Lei n° 12.780/2013 - Lei n° 12.780/2013, Art. 13"},
  {code: "160", group: "suspensao", description: "Regime Especial de Admissão Temporária nos Termos do Art. 2o da IN 1361/2013"},
  {code: "161", group: "suspensao", description: "Regime Especial de Admissão Temporária nos termos do art. 5o da IN 1361/2013"},
  {code: "162", group: "suspensao", description: "Regime Especial de Admissão Temporária nos termos do art. 7o da IN 1361/2013 (Suspensão com pagamento de tributos diferidos até a duração do regime, limitado a 100% do valor original)"},
  {code: "163", group: "suspensao", description: "REPETRO-Industrialização Venda no mercado interno de matérias-primas, produtos intermediários e materiais de embalagem para serem utilizados integralmente no processo de industrialização de produto final destinado às atividades de exploração, de desenvolvimento e de produção de petróleo, de gás natural e de outros hidrocarbonetos fluidos à PJ habilitada no Repetro-Industrialização. - Instrução Normativa RFB nº 1901, de 17 de julho de 2019."},
  {code: "164", group: "suspensao", description: "REPETRO-SPED Venda dos produtos finais destinados às atividades de exploração, de desenvolvimento e de produção de petróleo, de gás natural e de outros hidrocarbonetos fluidos previstas na Lei nº 9.478, de 6 de agosto de 1997 , na Lei nº 12.276, de 30 de junho de 2010, e na Lei nº 12.351, de 22 de dezembro de 2010, por fabricantes desses, beneficiários do Repetro-Industrialização, quando diretamente adquiridos por pessoa jurídica habilitada no Repetro-Sped.- Instrução Normativa RFB nº 1901, de 17 de julho de 2019."},
  {code: "165", group: "suspensao", description: "O industrial ou equiparado, mediante requerimento, nas operações anteriores, concomitantes ou posteriores às saídas que promover, nas hipóteses e condições estabelecidas pela Secretaria da Receita Federal, nos termos da IN RFB nº 1.081/2010."},
  {code: "301", group: "isencao", description: "Produtos industrializados por instituições de educação ou de assistência social, destinados a uso próprio ou a distribuição gratuita a seus educandos ou assistidos - Art. 54 Inciso I do Decreto 7.212/2010"},
  {code: "302", group: "isencao", description: "Produtos industrializados por estabelecimentos públicos e autárquicos da União, dos Estados, do Distrito Federal e dos Municípios, não destinados a comércio - Art. 54 Inciso II do Decreto 7.212/2010"},
  {code: "303", group: "isencao", description: "Amostras de produtos para distribuição gratuita, de diminuto ou nenhum valor comercial -Art. 54 Inciso III do Decreto 7.212/2010"},
  {code: "304", group: "isencao", description: "Amostras de tecidos sem valor comercial- Art. 54 Inciso IV do Decreto 7.212/2010"},
  {code: "305", group: "isencao", description: "Pés isolados de calçados - Art. 54 Inciso V do Decreto 7.212/2010"},
  {code: "306", group: "isencao", description: "Aeronaves de uso militar e suas partes e peças, vendidas à União - Art. 54 Inciso VI do Decreto 7.212/2010"},
  {code: "307", group: "isencao", description: "Caixões funerários - Art. 54 Inciso VII do Decreto 7.212/2010"},
  {code: "308", group: "isencao", description: "Papel destinado à impressão de músicas - Art. 54 Inciso VIII do Decreto 7.212/2010"},
  {code: "309", group: "isencao", description: "Panelas e outros artefatos semelhantes, de uso doméstico, de fabricação rústica, de pedra ou barro bruto - Art. 54 Inciso IX do Decreto 7.212/2010"},
  {code: "310", group: "isencao", description: "Chapéus, roupas e proteção, de couro, próprios para tropeiros - Art. 54 Inciso X do Decreto 7.212/2010"},
  {code: "311", group: "isencao", description: "Material bélico, de uso privativo das Forças Armadas, vendido à União - Art. 54 Inciso XI do Decreto 7.212/2010"},
  {code: "312", group: "isencao", description: "Automóvel adquirido diretamente a fabricante nacional, pelas missões diplomáticas e repartições consulares de caráter permanente, ou seus integrantes, bem assim pelas representações internacionais ou regionais de que o Brasil seja membro, e seus funcionários, peritos, técnicos e consultores, de nacionalidade estrangeira, que exerçam funções de caráter permanente - Art. 54 Inciso XII do Decreto 7.212/2010"},
  {code: "313", group: "isencao", description: "Veículo de fabricação nacional adquirido por funcionário das missões diplomáticas acreditadas junto ao Governo Brasileiro - Art. 54 Inciso XIII do Decreto 7.212/2010"},
  {code: "314", group: "isencao", description: "Produtos nacionais saídos diretamente para Lojas Francas - Art. 54 Inciso XIV do Decreto 7.212/2010"},
  {code: "315", group: "isencao", description: "Materiais e equipamentos destinados a Itaipu Binacional - Art. 54 Inciso XV do Decreto 7.212/2010"},
  {code: "316", group: "isencao", description: "Produtos Importados por missões diplomáticas, consulados ou organismo internacional -Art. 54 Inciso XVI do Decreto 7.212/2010"},
  {code: "317", group: "isencao", description: "Bagagem de passageiros desembaraçada com isenção do II. - Art. 54 Inciso XVII do Decreto 7.212/2010"},
  {code: "318", group: "isencao", description: "Bagagem de passageiros desembaraçada com pagamento do II. - Art. 54 Inciso XVIII do Decreto 7.212/2010"},
  {code: "319", group: "isencao", description: "Remessas postais internacionais sujeitas a tributação simplificada. - Art. 54 Inciso XIX do Decreto 7.212/2010"},
  {code: "320", group: "isencao", description: "Máquinas e outros destinados à pesquisa científica e tecnológica - Art. 54 Inciso XX do Decreto 7.212/2010"},
  {code: "321", group: "isencao", description: "Produtos de procedência estrangeira, isentos do II conforme Lei n° 8032/1990. - Art. 54 Inciso XXI do Decreto 7.212/2010"},
  {code: "322", group: "isencao", description: "Produtos de procedência estrangeira utilizados em eventos esportivos - Art. 54 Inciso XXII do Decreto 7.212/2010"},
  {code: "323", group: "isencao", description: "Veículos automotores, máquinas, equipamentos, bem assim suas partes e peças separadas, destinadas à utilização nas atividades dos Corpos de Bombeiros - Art. 54 Inciso XXIII do Decreto 7.212/2010"},
  {code: "324", group: "isencao", description: "Produtos importados para consumo em congressos, feiras e exposições - Art. 54 Inciso XXIV do Decreto 7.212/2010"},
  {code: "325", group: "isencao", description: "Bens de informática, Matéria Prima, produtos intermediários e embalagem destinados a Urnas eletrônicas - TSE - Art. 54 Inciso XXV do Decreto 7.212/2010"},
  {code: "326", group: "isencao", description: "Materiais, equipamentos, máquinas, aparelhos e instrumentos, bem assim os respectivos acessórios, sobressalentes e ferramentas, que os acompanhem, destinados à construção do Gasoduto Brasil - Bolívia - Art. 54 Inciso XXVI do Decreto 7.212/2010"},
  {code: "327", group: "isencao", description: "Partes, peças e componentes, adquiridos por estaleiros navais brasileiros, destinados ao emprego na conservação, modernização, conversão ou reparo de embarcações registradas no Registro Especial Brasileiro - REB - Art. 54 Inciso XXVII do Decreto 7.212/2010"},
  {code: "328", group: "isencao", description: "Aparelhos transmissores e receptores de radiotelefonia e radiotelegrafia; veículos para patrulhamento policial; armas e munições, destinados a órgãos de segurança pública da União, dos Estados e do Distrito Federal - Art. 54 Inciso XXVIII do Decreto 7.212/2010"},
  {code: "329", group: "isencao", description: "Automóveis de passageiros de fabricação nacional destinados à utilização como táxi adquiridos por motoristas profissionais - Art. 55 Inciso I do Decreto 7.212/2010"},
  {code: "330", group: "isencao", description: "Automóveis de passageiros de fabricação nacional destinados à utilização como táxi por impedidos de exercer atividade por destruição, furto ou roubo do veículo adquiridos por motoristas profissionais. - Art. 55 Inciso II do Decreto 7.212/2010"},
  {code: "331", group: "isencao", description: "Automóveis de passageiros de fabricação nacional destinados à utilização como táxi adquiridos por cooperativas de trabalho. - Art. 55 Inciso II do Decreto 7.212/2010"},
  {code: "332", group: "isencao", description: "Automóveis de passageiros de fabricação nacional, destinados a pessoas portadoras de deficiência física, visual, mental severa ou profunda, ou autistas - Art. 55 Inciso IV do Decreto 7.212/2010"},
  {code: "333", group: "isencao", description: "Produtos estrangeiros, recebidos em doação de representações diplomáticas estrangeiras sediadas no País,vendidos em feiras, bazares e eventos semelhantes porentidades beneficentes - Art. 67 do Decreto 7.212/2010"},
  {code: "334", group: "isencao", description: "Produtos industrializados na Zona Franca de Manaus - ZFM, destinados ao seu consumo interno - Art. 81 Inciso I do Decreto 7.212/2010"},
  {code: "335", group: "isencao", description: "Produtos industrializados na ZFM, por estabelecimentos com projetos aprovados pela SUFRAMA, destinados a comercialização em qualquer outro ponto do Território Nacional -Art. 81 Inciso II do Decreto 7.212/2010"},
  {code: "336", group: "isencao", description: "Produtos nacionais destinados à entrada na ZFM, para seu consumo interno, utilização ou industrialização, ou ainda, para serem remetidos, por intermédio de seus entrepostos, à Amazônia Ocidental - Art. 81 Inciso III do Decreto 7.212/2010"},
  {code: "337", group: "isencao", description: "Produtos industrializados por estabelecimentos com projetos aprovados pela SUFRAMA, consumidos ou utilizados na Amazônia Ocidental,ou adquiridos através da ZFM ou de seus entrepostos na referida região - Art. 95 Inciso I do Decreto 7.212/2010"},
  {code: "338", group: "isencao", description: "Produtos de procedência estrangeira, relacionados na legislação, oriundos da ZFM e que derem entrada na Amazônia Ocidental para ali serem consumidos ou utilizados:- Art. 95 Inciso II do Decreto 7.212/2010"},
  {code: "339", group: "isencao", description: "Produtos elaborados com matérias-primas agrícolas e extrativas vegetais de produção regional, por estabelecimentos industriais localizados na Amazônia Ocidental, com projetos aprovados pela SUFRAMA - Art. 95 Inciso III do Decreto 7.212/2010"},
  {code: "340", group: "isencao", description: "Produtos industrializados em Área de Livre Comércio - Art. 105 do Decreto 7.212/2010"},
  {code: "341", group: "isencao", description: "Produtos nacionais ou nacionalizados, destinados à entrada na Área de Livre Comércio de Tabatinga - ALCT - Art. 107 do Decreto 7.212/2010"},
  {code: "342", group: "isencao", description: "Produtos nacionais ou nacionalizados, destinados à entrada na Área de Livre Comércio de Guajará-Mirim - ALCGM - Art. 110 do Decreto 7.212/2010"},
  {code: "343", group: "isencao", description: "Produtos nacionais ou nacionalizados, destinados à entrada nas Áreas de Livre Comércio de Boa Vista - ALCBV e Bonfim - ALCB - Art. 113 do Decreto 7.212/2010"},
  {code: "344", group: "isencao", description: "Produtos nacionais ou nacionalizados, destinados à entrada na Área de Livre Comércio de Macapá e Santana - ALCMS - Art. 117 do Decreto 7.212/2010"},
  {code: "345", group: "isencao", description: "Produtos nacionais ou nacionalizados, destinados à entrada nas Áreas de Livre Comércio de Brasiléia - ALCB e de Cruzeiro do Sul - ALCCS - Art. 120 do Decreto 7.212/2010"},
  {code: "346", group: "isencao", description: "Recompe - equipamentos de informática - de beneficiário do regime para escolas das redes públicas de ensino federal, estadual, distrital, municipal ou nas escolas sem fins lucrativos de atendimento a pessoas com deficiência - Decreto n° 7.243/2010, art. 7°"},
  {code: "347", group: "isencao", description: "Rio 2016 - Importação de materiais para os jogos (medalhas, troféus, impressos, bens não duráveis, etc.) - Lei n° 12.780/2013, Art. 4°, §1°, I"},
  {code: "348", group: "isencao", description: "Rio 2016 - Suspensão convertida em Isenção - Lei n° 12.780/2013, Art. 6°, I"},
  {code: "349", group: "isencao", description: "Rio 2016 - Empresas vinculadas ao CIO - Lei n° 12.780/2013, Art. 9°, I, d"},
  {code: "350", group: "isencao", description: "Rio 2016 - Saída de produtos importados pelo RIO 2016- Lei n° 12.780/2013, Art. 10, I, d"},
  {code: "351", group: "isencao", description: "Rio 2016 - Produtos nacionais, não duráveis, uso e consumo dos eventos, adquiridos pelas pessoas jurídicas mencionadas no § 2o do art. 4o da Lei n° 12.780/2013, Art. 12"},
  {code: "601", group: "reducao", description: "Equipamentos e outros destinados à pesquisa e ao desenvolvimento tecnológico - Art. 72 do Decreto 7.212/2010"},
  {code: "602", group: "reducao", description: "Equipamentos e outros destinados à empresas habilitadas no PDTI e PDTA utilizados em pesquisa e ao desenvolvimento tecnológico - Art. 73 do Decreto 7.212/2010"},
  {code: "603", group: "reducao", description: "Microcomputadores e outros de até R$11.000,00, unidades de disco, circuitos, etc, destinados a bens de informática ou automação. Centro-Oeste SUDAM SUDENE - Art. 142, I do Decreto 7.212/2010"},
  {code: "604", group: "reducao", description: "Microcomputadores e outros de até R$11.000,00, unidades de disco, circuitos, etc, destinados a bens de informática ou automação. - Art. 142, I do Decreto 7.212/2010"},
  {code: "605", group: "reducao", description: "Bens de informática não incluídos no art. 142 do Decreto 7.212/2010 - Produzidos no Centro- Oeste, SUDAM, SUDENE - Art. 143, I do Decreto 7.212/2010"},
  {code: "606", group: "reducao", description: "Bens de informática não incluídos no art. 142 do Decreto 7.212/2010 - Art. 143, II do Decreto 7.212/2010"},
  {code: "607", group: "reducao", description: "Padis - Art. 150 do Decreto 7.212/2010"},
  {code: "608", group: "reducao", description: "Patvd - Art. 158 do Decreto 7.212/2010"},
  {code: "999", group: "outros", description: "Tributação normal IPI; Outros"},
]

/** Enquadramento genérico: tributação normal. É o default do leiaute. */
export const IPI_CENQ_DEFAULT = '999'

/** Faixa de enquadramento exigida por CST de IPI (RV W16-10). */
const CENQ_GROUP_BY_CST: Record<string, CEnqGroup> = {
  '04': 'imunidade', '54': 'imunidade',
  '05': 'suspensao', '55': 'suspensao',
  '02': 'isencao', '52': 'isencao',
}

const toOption = (e: CEnqEntry) => ({value: e.code, label: `${e.code} - ${e.description}`})

export const IPI_CENQ_OPTIONS = IPI_CENQ.map(toOption)

/**
 * Enquadramentos aceitos para o CST informado. Sem CST, devolve a tabela
 * inteira; com CST fora do mapa, só as opções de redução e o genérico.
 */
export function cEnqOptionsForCst(cst?: string): { value: string; label: string }[] {
  if (!cst) return IPI_CENQ_OPTIONS
  const group = CENQ_GROUP_BY_CST[cst]
  if (group) return IPI_CENQ.filter((e) => e.group === group).map(toOption)
  return IPI_CENQ.filter((e) => e.group === 'reducao' || e.group === 'outros').map(toOption)
}
