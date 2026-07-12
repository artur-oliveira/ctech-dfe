import {FileText, Receipt, Route, Truck} from 'lucide-react'

interface DfeIconProps {
  width?: number
  height?: number
}

/** NF-e — Nota Fiscal Eletrônica (sales invoice) */
export const NfeIcon = ({width = 16, height = 16}: DfeIconProps) => (
  <FileText width={width} height={height}/>
)

/** NFC-e — Nota Fiscal de Consumidor Eletrônica (POS receipt) */
export const NfceIcon = ({width = 16, height = 16}: DfeIconProps) => (
  <Receipt width={width} height={height}/>
)

/** CT-e — Conhecimento de Transporte Eletrônico (transport doc) */
export const CteIcon = ({width = 16, height = 16}: DfeIconProps) => (
  <Truck width={width} height={height}/>
)

/** MDF-e — Manifesto de Documento Fiscal Eletrônico (transport manifest) */
export const MdfeIcon = ({width = 16, height = 16}: DfeIconProps) => (
  <Route width={width} height={height}/>
)
