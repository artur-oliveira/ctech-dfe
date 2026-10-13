import * as cdk from 'aws-cdk-lib';
import {Match, Template} from 'aws-cdk-lib/assertions';
import {EventBusStack} from '../lib/event-bus-stack';

const synth = () => Template.fromStack(new EventBusStack(new cdk.App(), 'TestEventBus', {
    env: {account: '868899309401', region: 'us-east-1'},
    environment: 'dev',
}));

describe('EventBusStack — fila da exclusão de conta', () => {
    test('fila com DLQ, visibilidade de 30 min e 5 recebimentos', () => {
        synth().hasResourceProperties('AWS::SQS::Queue', {
            QueueName: 'dev-ctech-dfe-erasure',
            VisibilityTimeout: 1800,
            RedrivePolicy: Match.objectLike({maxReceiveCount: 5}),
        });
        synth().hasResourceProperties('AWS::SQS::Queue', {QueueName: 'dev-ctech-dfe-erasure-dlq'});
    });

    test('assina o tópico do ctech-account filtrando services=dfe no atributo, entrega raw', () => {
        const subs = synth().findResources('AWS::SNS::Subscription', {
            Properties: {Protocol: 'sqs', FilterPolicy: {services: ['dfe']}},
        });
        const props = Object.values(subs).map((r: any) => r.Properties);
        expect(props).toHaveLength(1);
        expect(props[0].RawMessageDelivery).toBe(true);
        expect(props[0].FilterPolicyScope ?? 'MessageAttributes').toBe('MessageAttributes');
    });
});
