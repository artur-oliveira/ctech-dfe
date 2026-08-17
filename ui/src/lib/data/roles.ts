export type RoleName = 'OWNER' | 'ADMIN' | 'USER' | 'VIEWER';

export const ROLE_OWNER: RoleName = 'OWNER'
export const ROLE_ADMIN: RoleName = 'ADMIN'

export const ROLE_LABEL: Record<RoleName, string> = {
    OWNER: 'Proprietário',
    ADMIN: 'Administrador',
    USER: 'Operador',
    VIEWER: 'Apenas leitura',
}

export const ASSIGNABLE_ROLES: { value: RoleName, label: string }[] = ['ADMIN', 'USER', 'VIEWER'].map((it: string) => {
    const value: RoleName = it as RoleName;
    return {
        value,
        label: (ROLE_LABEL[value] || '').toString(),
    }
})