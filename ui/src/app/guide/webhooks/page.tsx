'use client';

import {GuideBullets, GuideCallout, GuidePage, GuideSteps, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

const CODE = 'overflow-x-auto rounded-lg bg-gray-900 p-4 text-xs leading-relaxed text-gray-100';

export default function GuideWebhooks() {
  return (
    <GuidePage
      currentHref="/guide/webhooks"
      title="Webhooks"
      description="Em vez de consultar o status de cada documento, deixe o seu sistema ser avisado. Cada resultado chega assinado, com histórico de entregas e reenvio."
      sections={[
        {
          id: 'register',
          title: 'Cadastrar um webhook',
          summary: 'Você informa o endereço do seu sistema; o DF-e confirma que ele responde antes de ativar.',
          image: {
            src: '/guide/webhooks.webp',
            alt: 'Lista de webhooks da organização, com endereço, estado ativo ou desativado, eventos, empresas e última entrega',
          },
          body: (
            <>
              <GuideSteps>
                <li>Em <b>Configurações</b>, abra <b>Webhooks</b> e clique em <b>Novo webhook</b>.</li>
                <li>Informe o endereço (https). Marque os eventos e as empresas que interessam; sem nada marcado, vale tudo.</li>
                <li>Clique em <b>Cadastrar e validar</b>. Fazemos uma chamada de verificação ao seu endereço e só ativamos se ele responder certo.</li>
                <li>Guarde o <b>segredo de assinatura</b> que aparece. Ele é mostrado uma única vez.</li>
              </GuideSteps>
              <p>
                A verificação é um <b>GET</b> com <b>hub.mode=subscribe</b> e <b>hub.challenge</b> na URL. O seu sistema
                responde <b>HTTP 200</b> com o valor de <b>hub.challenge</b> como corpo inteiro, sem mais nada.
              </p>
              <GuideBullets>
                <li>O endereço precisa ser um site público na internet; endereços internos, <b>localhost</b> e IPs privados são recusados.</li>
                <li>Cada organização tem até <b>5 webhooks</b>, e só proprietário e administrador os gerenciam.</li>
                <li>Trocar o segredo gera outro, e o antigo para de valer na hora.</li>
              </GuideBullets>
              <GuideCallout kind="info" title="Escolher, não digitar">
                Eventos e empresas são escolhidos em caixas de marcar; não há campo livre para digitar nomes de evento.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'add',
          title: 'O formulário e o segredo',
          summary: 'Erros do cadastro dizem o que o seu sistema precisa corrigir.',
          image: {
            src: '/guide/webhooks-add.webp',
            alt: 'Painel Novo webhook, com o endereço, os eventos Autorizada, Rejeitada e Falha na emissão, e as empresas da organização',
          },
          body: (
            <>
              <p>
                Se a verificação falhar, a tela mostra o motivo: o endereço não respondeu, respondeu com outro status ou
                respondeu sem devolver o código. Nada é gravado até a verificação passar.
              </p>
              <GuideCallout kind="warning" title="O segredo não volta">
                Guarde-o em um cofre de segredos do seu sistema. Se perder, use <b>Trocar segredo</b> no webhook e atualize o seu sistema.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'payload',
          title: 'O que chega no seu sistema',
          summary: 'O mínimo para você saber que algo aconteceu; o resto você consulta pela API.',
          body: (
            <>
              <pre className={CODE}>{`POST https://seu-sistema.com.br/hooks/dfe
X-Ctech-Event-Id: 01JC3F7K2N9P4Q6S8T0V1W3X5Z
X-Ctech-Event-Type: document.rejected
X-Ctech-Timestamp: 1763125331
X-Ctech-Signature: v1=9f2c...e1

{
  "id": "01JC3F7K2N9P4Q6S8T0V1W3X5Z",
  "type": "document.rejected",
  "occurred_at": "2026-11-14T13:02:11Z",
  "data": {
    "id": "35250512345678000195550010000000011000000015",
    "access_key": "35250512345678000195550010000000011000000015",
    "status": "rejected",
    "document_type": "nfe",
    "rejection": {"code": "225", "message": "Falha no Schema XML"}
  }
}`}</pre>
              <GuideTerms>
                <GuideTerm term="data.id">A chave do documento no DF-e: a chave de acesso (NF-e, NFC-e, CT-e e MDF-e) ou o id da DPS (NFS-e).</GuideTerm>
                <GuideTerm term="data.access_key">A chave de acesso. A NFS-e não a traz aqui, porque a chave dela só existe depois da autorização.</GuideTerm>
                <GuideTerm term="data.status">authorized, rejected ou failed (a falha na emissão).</GuideTerm>
                <GuideTerm term="data.document_type">nfe, nfce, cte, mdfe ou nfse: de qual API ler o documento.</GuideTerm>
                <GuideTerm term="data.rejection">Só na rejeição: o código e o motivo da autoridade.</GuideTerm>
              </GuideTerms>
              <p>
                Valores, destinatário e itens ficam de fora de propósito: uma URL digitada errada vaza um identificador, não um documento
                fiscal. Para ver o documento, consulte a API com a sua credencial.
              </p>
              <GuideCallout kind="warning" title="Pode chegar mais de uma vez">
                A entrega é <b>pelo menos uma vez</b>: se a sua resposta demorar, repetimos. Use o cabeçalho <b>X-Ctech-Event-Id</b> para ignorar o que já tratou.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'verify',
          title: 'Conferir a assinatura',
          summary: 'Quem não conhece o segredo não consegue forjar um aviso.',
          body: (
            <>
              <p>
                A assinatura é o <b>HMAC-SHA256</b>, em hexadecimal, de <b>timestamp + &quot;.&quot; + corpo</b>, com o seu segredo, e vem
                como <b>v1=&lt;hex&gt;</b> em <b>X-Ctech-Signature</b>. Recuse o aviso se a assinatura não bater ou se o timestamp
                estiver a mais de 5 minutos de agora.
              </p>
              <pre className={CODE}>{`// Node.js
import {createHmac, timingSafeEqual} from 'node:crypto';

function valid(secret, timestamp, rawBody, header) {
  const age = Math.abs(Date.now() / 1000 - Number(timestamp));
  if (!(age <= 300)) return false;
  const want = 'v1=' + createHmac('sha256', secret).update(timestamp + '.' + rawBody).digest('hex');
  return header.length === want.length && timingSafeEqual(Buffer.from(header), Buffer.from(want));
}`}</pre>
              <GuideCallout kind="info" title="Use o corpo exato">
                Calcule sobre os bytes recebidos, antes de converter para objeto. Reescrever o JSON muda a assinatura.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'deliveries',
          title: 'Entregas, tentativas e reenvio',
          summary: 'Cada tentativa fica registrada, e uma entrega pode ser repetida.',
          image: {
            src: '/guide/webhooks-deliveries.webp',
            alt: 'Entregas de um webhook, com tipo do evento, estado, número de tentativas e botões Detalhes e Reenviar',
          },
          body: (
            <>
              <p>
                Responda com qualquer <b>2xx</b> para confirmar. Em caso de erro, tentamos de novo com espera crescente (30 segundos,
                1 minuto, 2 minutos, e assim por diante, até 12 horas), no máximo <b>8 vezes</b>, o que cobre cerca de dois dias.
              </p>
              <GuideBullets>
                <li><b>Testar</b> envia um evento <b>ping</b> só para aquele webhook, ignorando os filtros, e mostra o resultado em Entregas.</li>
                <li><b>Detalhes</b> abre cada tentativa: status HTTP, duração e os primeiros 512 caracteres da resposta.</li>
                <li><b>Reenviar</b> volta a entrega para a fila e tenta na hora. Não funciona em webhook desativado.</li>
                <li>Depois de <b>12 falhas seguidas</b>, o webhook é <b>desativado</b> e avisamos a equipe. Corrija o seu sistema, remova o webhook e cadastre-o de novo.</li>
                <li>As entregas ficam guardadas por 90 dias.</li>
              </GuideBullets>
            </>
          ),
        },
      ]}
    />
  );
}
