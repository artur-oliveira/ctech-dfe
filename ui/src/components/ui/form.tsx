'use client'

import * as React from 'react'
import {
  Controller,
  type ControllerProps,
  type FieldPath,
  type FieldValues,
  FormProvider,
  useFormContext,
} from 'react-hook-form'
import {Label} from '@/components/ui/label'
import {cn} from '@/lib/utils'

export const Form = FormProvider

type FormFieldContextValue<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = { name: TName }

const FormFieldContext = React.createContext<FormFieldContextValue>(
  {} as FormFieldContextValue
)

export function FormField<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({...props}: ControllerProps<TFieldValues, TName>) {
  return (
    <FormFieldContext.Provider value={{name: props.name}}>
      <Controller {...props} />
    </FormFieldContext.Provider>
  )
}

type FormItemContextValue = { id: string }

const FormItemContext = React.createContext<FormItemContextValue>(
  {} as FormItemContextValue
)

export function useFormField() {
  const fieldContext = React.useContext(FormFieldContext)
  const itemContext = React.useContext(FormItemContext)
  const {getFieldState, formState} = useFormContext()
  const fieldState = getFieldState(fieldContext.name, formState)
  return {
    id: itemContext.id,
    name: fieldContext.name,
    formItemId: `${itemContext.id}-form-item`,
    formDescriptionId: `${itemContext.id}-form-item-description`,
    formMessageId: `${itemContext.id}-form-item-message`,
    ...fieldState,
  }
}


/**
 * Props ARIA do campo, quando ele vive dentro de um formulário react-hook-form.
 * O `FormMessage` já renderiza um id de mensagem desde sempre, e nenhum controle
 * o consumia: o erro pintava em vermelho para quem enxerga e não existia para
 * quem usa leitor de tela. Fora de um formulário, devolve {} e o controle segue
 * igual.
 */
export function useFieldAria(name?: string): {
  'aria-invalid'?: true
  'aria-describedby'?: string
} {
  const form = useFormContext()
  const errors = form?.formState.errors
  if (!form || !name || !errors) return {}
  const hasError = name.split('.').reduce<unknown>(
    (node, key) => (node && typeof node === 'object' ? (node as Record<string, unknown>)[key] : undefined),
    errors,
  )
  if (!hasError) return {}
  return {'aria-invalid': true, 'aria-describedby': `${name}-form-item-message`}
}

export function FormItem({className, ...props}: React.ComponentProps<'div'>) {
  const id = React.useId()
  return (
    <FormItemContext.Provider value={{id}}>
      <div data-slot="form-item" className={cn('grid gap-1', className)} {...props} />
    </FormItemContext.Provider>
  )
}

export function FormLabel({
                            className,
                            htmlFor,
                            ...props
                          }: React.ComponentProps<typeof Label>) {
  const {error, name} = useFormField()
  // Controls under a FormField set id={field.name}, so associate the label with
  // that id (the generated formItemId is not applied to any element here).
  // An explicit htmlFor prop still wins; falls back to undefined (no broken for)
  // when used outside a FormField.
  return (
    <Label
      data-slot="form-label"
      className={cn(error && 'text-destructive', className)}
      htmlFor={htmlFor ?? name ?? undefined}
      {...props}
    />
  )
}

export function FormDescription({
                                  className,
                                  ...props
                                }: React.ComponentProps<'p'>) {
  const {formDescriptionId} = useFormField()
  return (
    <p
      id={formDescriptionId}
      data-slot="form-description"
      className={cn('text-muted-foreground text-[0.8rem]', className)}
      {...props}
    />
  )
}

export function FormMessage({
                              className,
                              children,
                              ...props
                            }: React.ComponentProps<'p'>) {
  const {error, name} = useFormField()
  // O id vem do nome do campo, não do useId do FormItem: é assim que o controle
  // consegue apontar para a mensagem sem precisar de um wrapper por campo.
  const formMessageId = name ? `${name}-form-item-message` : undefined
  const body = error ? String(error.message ?? '') : children
  if (!body) return null
  return (
    <p
      id={formMessageId}
      data-slot="form-message"
      className={cn('text-destructive text-[0.8rem] font-medium', className)}
      {...props}
    >
      {body}
    </p>
  )
}
