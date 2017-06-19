/*
 * Copyright 2017 Google Inc.
 * 
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 * 
 *     http://www.apache.org/licenses/LICENSE-2.0
 * 
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package io.vitess.jdbc;

import io.vitess.client.Context;
import io.vitess.client.RpcClient;
import io.vitess.client.VTGateConn;
import io.vitess.client.grpc.GrpcClientFactory;
import io.vitess.client.grpc.RetryingInterceptorConfig;
import io.vitess.client.grpc.tls.TlsOptions;
import io.vitess.util.CommonUtils;
import io.vitess.util.Constants;
import java.io.IOException;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Created by naveen.nahata on 24/02/16.
 */
public class VitessVTGateManager {
    /*
    Current implementation have one VTGateConn for ip-port-username combination
    */
    private static ConcurrentHashMap<String, VTGateConn> vtGateConnHashMap =
        new ConcurrentHashMap<>();


    /**
     * VTGateConnections object consist of vtGateIdentifire list and return vtGate object in round robin.
     */
    public static class VTGateConnections implements VTGateConnectionProvider {
        private List<String> vtGateIdentifiers = new ArrayList<>();
        int counter;

        @Override public void init(VitessJDBCUrl vitessJDBCUrl)
            throws SQLException {
            for (VitessJDBCUrl.HostInfo hostInfo : vitessJDBCUrl.getHostInfos()) {
                String identifier = getIdentifer(hostInfo.getHostname(), hostInfo.getPort(), vitessJDBCUrl.getUsername(), vitessJDBCUrl.getKeyspace());
                synchronized (VitessVTGateManager.class) {
                    if (!vtGateConnHashMap.containsKey(identifier)) {
                        updateVtGateConnHashMap(identifier, hostInfo, vitessJDBCUrl);
                    }
                }
                vtGateIdentifiers.add(identifier);
            }
            Random random = new Random();
            counter = random.nextInt(vtGateIdentifiers.size());
        }

        /**
         * Return VTGate Instance object.
         *
         * @return
         */
        public VTGateConn connect() {
            counter++;
            counter = counter % vtGateIdentifiers.size();
            return vtGateConnHashMap.get(vtGateIdentifiers.get(counter));
        }

    }

    private static String getIdentifer(String hostname, int port, String userIdentifer, String keyspace) {
        return (hostname + port + userIdentifer + keyspace);
    }

    /**
     * Create VTGateConn and update vtGateConnHashMap.
     */
    private static void updateVtGateConnHashMap(String identifier, VitessJDBCUrl.HostInfo hostInfo,
        VitessJDBCUrl vitessJDBCUrl) {
        vtGateConnHashMap.put(identifier, getVtGateConn(hostInfo, vitessJDBCUrl));
    }

    /**
     * Create vtGateConn object with given identifier.
     */
    private static VTGateConn getVtGateConn(VitessJDBCUrl.HostInfo hostInfo, VitessJDBCUrl vitessJDBCUrl) {
        ConnectionProperties connection = vitessJDBCUrl.getConnectionProperties();
        final String username = vitessJDBCUrl.getUsername();
        final String keyspace = vitessJDBCUrl.getKeyspace();
        final Context context = CommonUtils.createContext(username, Constants.CONNECTION_TIMEOUT);
        RetryingInterceptorConfig retryingConfig = getRetryingInterceptorConfig(connection);
        RpcClient client;
        if (connection.getUseSSL()) {
            final String keyStorePath = connection.getKeyStore() != null
                    ? connection.getKeyStore() : System.getProperty(Constants.Property.KEYSTORE_FULL);
            final String keyStorePassword = connection.getKeyStorePassword() != null
                    ? connection.getKeyStorePassword() : System.getProperty(Constants.Property.KEYSTORE_PASSWORD_FULL);
            final String keyAlias = connection.getKeyAlias() != null
                    ? connection.getKeyAlias() : System.getProperty(Constants.Property.KEY_ALIAS_FULL);
            final String keyPassword = connection.getKeyPassword() != null
                    ? connection.getKeyPassword() : System.getProperty(Constants.Property.KEY_PASSWORD_FULL);
            final String trustStorePath = connection.getTrustStore() != null
                    ? connection.getTrustStore() : System.getProperty(Constants.Property.TRUSTSTORE_FULL);
            final String trustStorePassword = connection.getTrustStorePassword() != null
                    ? connection.getTrustStorePassword() : System.getProperty(Constants.Property.TRUSTSTORE_PASSWORD_FULL);
            final String trustAlias = connection.getTrustAlias() != null
                    ? connection.getTrustAlias() : System.getProperty(Constants.Property.TRUST_ALIAS_FULL);

            final TlsOptions tlsOptions = new TlsOptions()
                    .keyStorePath(keyStorePath)
                    .keyStorePassword(keyStorePassword)
                    .keyAlias(keyAlias)
                    .keyPassword(keyPassword)
                    .trustStorePath(trustStorePath)
                    .trustStorePassword(trustStorePassword)
                    .trustAlias(trustAlias);

            client = new GrpcClientFactory(retryingConfig).createTls(context, hostInfo.toString(), tlsOptions);
        } else {
            client = new GrpcClientFactory(retryingConfig).create(context, hostInfo.toString());
        }
        if (null == keyspace) {
            return (new VTGateConn(client));
        }
        return (new VTGateConn(client, keyspace));
    }

    private static RetryingInterceptorConfig getRetryingInterceptorConfig(ConnectionProperties conn) {
        if (!conn.getGrpcRetriesEnabled()) {
            return RetryingInterceptorConfig.noOpConfig();
        }

        return RetryingInterceptorConfig.exponentialConfig(conn.getGrpcRetryInitialBackoffMillis(), conn.getGrpcRetryMaxBackoffMillis(), conn.getGrpcRetryBackoffMultiplier());
    }

    public static void close() throws SQLException {
        SQLException exception = null;

        for (VTGateConn vtGateConn : vtGateConnHashMap.values()) {
            try {
                vtGateConn.close();
            } catch (IOException e) {
                exception = new SQLException(e.getMessage(), e);
            }
        }
        vtGateConnHashMap.clear();
        if (null != exception) {
            throw exception;
        }
    }
}
