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

import java.io.IOException;
import java.lang.reflect.Field;
import java.sql.SQLException;
import java.util.Properties;
import java.util.concurrent.ConcurrentHashMap;

import org.joda.time.Duration;
import org.junit.Assert;
import org.junit.Test;

import io.vitess.client.Context;
import io.vitess.client.RpcClient;
import io.vitess.client.VTGateConn;
import io.vitess.client.grpc.GrpcClientFactory;
import io.vitess.proto.Vtrpc;

/**
 * Created by naveen.nahata on 29/02/16.
 */
public class VitessVTGateManagerTest {

    public VTGateConn getVtGateConn() {
        Vtrpc.CallerID callerId = Vtrpc.CallerID.newBuilder().setPrincipal("username").build();
        Context ctx =
            Context.getDefault().withDeadlineAfter(Duration.millis(500)).withCallerId(callerId);
        RpcClient client = new GrpcClientFactory().create(ctx, "host:80");
        return new VTGateConn(client);
    }

    @Test public void testVtGateConnectionsConstructorMultipleVtGateConnections()
        throws SQLException, NoSuchFieldException, IllegalAccessException, IOException {
        VitessVTGateManager.close();
        Properties info = new Properties();
        info.setProperty("username", "user");
        VitessJDBCUrl url = new VitessJDBCUrl(
                "jdbc:vitess://10.33.17.231:15991:xyz,10.33.17.232:15991:xyz,10.33.17"
                        + ".233:15991/shipment/shipment?tabletType=master", info);
        VitessVTGateManager.VTGateConnections vtGateConnections =
            new VitessVTGateManager.VTGateConnections();
        vtGateConnections.init(url);

        info.setProperty("username", "user");
        VitessJDBCUrl url1 = new VitessJDBCUrl(
            "jdbc:vitess://10.33.17.231:15991:xyz,10.33.17.232:15991:xyz,11.33.17"
                + ".233:15991/shipment/shipment?tabletType=master", info);
        VitessVTGateManager.VTGateConnections vtGateConnections1 =
            new VitessVTGateManager.VTGateConnections();
        vtGateConnections1.init(url1);

        Field privateMapField = VitessVTGateManager.class.
            getDeclaredField("vtGateConnHashMap");
        privateMapField.setAccessible(true);
        ConcurrentHashMap<String, VTGateConn> map =
            (ConcurrentHashMap<String, VTGateConn>) privateMapField.get(VitessVTGateManager.class);
        Assert.assertEquals(4, map.size());
        VitessVTGateManager.close();
    }

    @Test public void testVtGateConnectionsConstructor()
        throws SQLException, NoSuchFieldException, IllegalAccessException, IOException {
        VitessVTGateManager.close();
        Properties info = new Properties();
        info.setProperty("username", "user");
        VitessJDBCUrl url = new VitessJDBCUrl(
            "jdbc:vitess://10.33.17.231:15991:xyz,10.33.17.232:15991:xyz,10.33.17"
                + ".233:15991/shipment/shipment?tabletType=master", info);
        VitessVTGateManager.VTGateConnections vtGateConnections =
            new VitessVTGateManager.VTGateConnections();
        vtGateConnections.init(url);
        Assert.assertEquals(vtGateConnections.connect() instanceof VTGateConn, true);
        VTGateConn vtGateConn = vtGateConnections.connect();
        Field privateMapField = VitessVTGateManager.class.
            getDeclaredField("vtGateConnHashMap");
        privateMapField.setAccessible(true);
        ConcurrentHashMap<String, VTGateConn> map =
            (ConcurrentHashMap<String, VTGateConn>) privateMapField.get(VitessVTGateManager.class);
        Assert.assertEquals(3, map.size());
        VitessVTGateManager.close();
    }

}
