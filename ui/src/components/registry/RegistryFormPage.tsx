'use client';

import {Suspense, type ReactNode} from 'react';
import Link from 'next/link';
import {useRouter, useSearchParams} from 'next/navigation';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useAuth} from '@/lib/hooks/useAuth';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';

interface Shared<Out, Body> {
  listHref: string
  listLabel: string
  singular: string
  listKey: (orgPk: string | undefined) => readonly unknown[]
  /** Formulário do cadastro; `initialData` só vem na edição. */
  renderForm: (props: { initialData?: Out; onSubmit: (data: Body) => Promise<void>; loading: boolean }) => ReactNode
}

export interface RegistryNewPageProps<Out, Body> extends Shared<Out, Body> {
  create: (data: Body) => Promise<unknown>
}

export interface RegistryEditPageProps<Out, Body> extends Shared<Out, Body> {
  detailKey: (id: string) => readonly unknown[]
  fetchOne: (id: string) => Promise<Out>
  update: (id: string, data: Body) => Promise<unknown>
}

function Shell({listHref, listLabel, crumb, title, children}: { listHref: string; listLabel: string; crumb: string; title: string; children: ReactNode }) {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href={listHref} className="hover:text-brand-600">{listLabel}</Link>
          <span>/</span>
          <span className="text-gray-600">{crumb}</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">{title}</h1>
        {children}
      </div>
    </RootLayout>
  );
}

function NewContent<Out, Body>(p: RegistryNewPageProps<Out, Body>) {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const mutation = useMutation({
    mutationFn: (d: Body) => p.create(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: p.listKey(selectedOrg?.pk)});
      router.push(p.listHref);
    },
  });
  return (
    <Shell listHref={p.listHref} listLabel={p.listLabel} crumb={`Novo ${p.singular}`} title={`Novo ${p.singular}`}>
      {p.renderForm({onSubmit: async (d) => { await mutation.mutateAsync(d); }, loading: mutation.isPending})}
    </Shell>
  );
}

/** Página "novo" de um cadastro da organização. */
export function RegistryNewPage<Out, Body>(props: RegistryNewPageProps<Out, Body>) {
  return (
    <ProtectedRoute>
      <NewContent {...props}/>
    </ProtectedRoute>
  );
}

function EditContent<Out, Body>(p: RegistryEditPageProps<Out, Body>) {
  const id = useSearchParams().get('id') ?? '';
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const {data: entity, isLoading} = useQuery({queryKey: p.detailKey(id), queryFn: () => p.fetchOne(id), enabled: !!id && !!selectedOrg});
  const mutation = useMutation({
    mutationFn: (d: Body) => p.update(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: p.listKey(selectedOrg?.pk)});
      void qc.invalidateQueries({queryKey: p.detailKey(id)});
      router.push(p.listHref);
    },
  });
  return (
    <Shell listHref={p.listHref} listLabel={p.listLabel} crumb={`Editar ${p.singular}`} title={`Editar ${p.singular}`}>
      {isLoading ? (
        <LoadingSkeleton/>
      ) : !entity ? (
        <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          Cadastro não encontrado.
        </div>
      ) : (
        p.renderForm({initialData: entity, onSubmit: async (d) => { await mutation.mutateAsync(d); }, loading: mutation.isPending})
      )}
    </Shell>
  );
}

/** Página "editar" de um cadastro da organização (`?id=`). */
export function RegistryEditPage<Out, Body>(props: RegistryEditPageProps<Out, Body>) {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditContent {...props}/>
      </Suspense>
    </ProtectedRoute>
  );
}
